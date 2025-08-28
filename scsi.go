// https://github.com/usbarmory/armory-ums
//
// Copyright (c) The armory-ums authors. All Rights Reserved.
//
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

package main

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/usbarmory/tamago/dma"
	"github.com/usbarmory/tamago/soc/nxp/usb"
	"github.com/usbarmory/tamago/soc/nxp/usdhc"
)

const (
	// p65, 3. Direct Access Block commands (SPC-5 and SBC-4), SCSI Commands Reference Manual, Rev. J
	TEST_UNIT_READY  = 0x00
	REQUEST_SENSE    = 0x03
	INQUIRY          = 0x12
	MODE_SENSE_6     = 0x1a
	MODE_SENSE_10    = 0x5a
	READ_CAPACITY_10 = 0x25
	READ_10          = 0x28
	WRITE_10         = 0x2a
	REPORT_LUNS      = 0xa0

	// service actions
	SERVICE_ACTION   = 0x9e
	READ_CAPACITY_16 = 0x10

	// 04-349r1 SPC-3 MMC-5 Merge PREVENT ALLOW MEDIUM REMOVAL commands
	PREVENT_ALLOW_MEDIUM_REMOVAL = 0x1e

	// p33, 4.10, USB Mass Storage Class – UFI Command Specification Rev. 1.0
	READ_FORMAT_CAPACITIES = 0x23
)

const (
	// exactly 8 bytes required
	VendorID = "F-Secure"
	// exactly 16 bytes required
	ProductID = "USB armory Mk II"
	// exactly 4 bytes required
	ProductRevision = "1.00"
)

type writeOp struct {
	csw    *usb.CSW
	lun    int
	lba    int
	blocks int
	size   int
	addr   uint
	buf    []byte
}

// buffer for write commands (which spawn across multiple USB transfers)
var dataPending *writeOp

// p94, 3.6.2 Standard INQUIRY data, SCSI Commands Reference Manual, Rev. J
func inquiry(length int) (data []byte) {
	data = make([]byte, 5)

	// device connected, direct access block device
	data[0] = 0x00
	// Removable Media
	data[1] = 0x80
	// SPC-3 compliant
	data[2] = 0x05
	// response data format (only 2 is allowed)
	data[3] = 0x02
	// additional length
	data[4] = byte(length - 5)

	// unused or obsolete flags
	data = append(data, make([]byte, 3)...)

	data = append(data, []byte(VendorID)...)
	data = append(data, []byte(ProductID)...)
	data = append(data, []byte(ProductRevision)...)

	if length > len(data) {
		// pad up to requested transfer length
		data = append(data, make([]byte, length-len(data))...)
	} else {
		data = data[0:length]
	}

	return
}

// p56, 2.4.1.2 Fixed format sense data, SCSI Commands Reference Manual, Rev. J
func sense(length int) (data []byte, err error) {
	data = make([]byte, 18)

	// error code
	data[0] = 0x70
	// no specific sense key
	data[2] = 0x00
	// additional sense length
	data[7] = byte(len(data) - 1 - 7)
	// no additional sense code
	data[12] = 0x00
	// no additional sense qualifier

	if length < len(data) {
		return nil, fmt.Errorf("unsupported REQUEST_SENSE transfer length %d > %d", length, len(data))
	}

	return
}

// p111, 3.11 MODE SENSE(6) command, SCSI Commands Reference Manual, Rev. J
func modeSense(length int) (data []byte, err error) {
	// Unsupported, an empty response is returned on all requests.
	data = make([]byte, length)

	// p378, 5.3.3 Mode parameter header formats, SCSI Commands Reference Manual, Rev. J
	data[0] = byte(length)

	return
}

// p179, 3.33 REPORT LUNS command, SCSI Commands Reference Manual, Rev. J
func reportLUNs(length int) (data []byte, err error) {
	buf := new(bytes.Buffer)
	luns := len(cards)

	binary.Write(buf, binary.BigEndian, uint32(luns*8))
	buf.Write(make([]byte, 4))

	for lun := 0; lun < luns; lun++ {
		// The information conforms to the Logical Unit Address Method defined
		// in SCC-2, and supports only First Level addressing (for each LUN,
		// only the second byte is used and contains the assigned LUN)."
		buf.WriteByte(0x00)
		binary.Write(buf, binary.BigEndian, uint8(lun))
		buf.Write(make([]byte, 6))
	}

	data = buf.Bytes()

	if length < buf.Len() {
		data = data[0:length]
	}

	return
}

// p155, 3.22 READ CAPACITY (10) command, SCSI Commands Reference Manual, Rev. J
func readCapacity10(card *usdhc.USDHC) (data []byte, err error) {
	info := card.Info()

	if info.Blocks <= 0 {
		return nil, fmt.Errorf("invalid block count %d", info.Blocks)
	}

	buf := new(bytes.Buffer)

	binary.Write(buf, binary.BigEndian, uint32(info.Blocks)-1)
	binary.Write(buf, binary.BigEndian, uint32(info.BlockSize))

	return buf.Bytes(), nil
}

// p157, 3.23 READ CAPACITY (16) command, SCSI Commands Reference Manual, Rev. J
func readCapacity16(card *usdhc.USDHC, length int) (data []byte, err error) {
	info := card.Info()
	buf := new(bytes.Buffer)

	if info.Blocks <= 0 {
		return nil, fmt.Errorf("invalid block count %d", info.Blocks)
	}

	binary.Write(buf, binary.BigEndian, uint64(info.Blocks)-1)
	binary.Write(buf, binary.BigEndian, uint64(info.BlockSize))
	buf.Grow(32 - buf.Len())

	data = buf.Bytes()

	if length < buf.Len() {
		data = data[0:length]
	}

	return
}

// p33, 4.10, USB Mass Storage Class – UFI Command Specification Rev. 1.0
func readFormatCapacities(card *usdhc.USDHC) (data []byte, err error) {
	info := card.Info()
	buf := new(bytes.Buffer)

	// capacity list length
	binary.Write(buf, binary.BigEndian, uint32(8))
	// number of blocks
	binary.Write(buf, binary.BigEndian, uint32(info.Blocks))
	// descriptor code: formatted media | block length
	binary.Write(buf, binary.BigEndian, uint32(0b10<<24|info.BlockSize&0xffffff))

	return buf.Bytes(), nil
}

func read(card *usdhc.USDHC, lba int, blocks int) (err error) {
	addr, buf := dma.Reserve(blocks*card.Info().BlockSize, usb.DTD_PAGE_SIZE)

	err = card.ReadBlocks(lba, buf)

	if err != nil {
		dma.Release(addr)
		return
	}

	send <- buf

	return
}

func write(card *usdhc.USDHC, lba int, buf []byte) (err error) {
	return card.WriteBlocks(lba, buf)
}

func handleCDB(cmd [16]byte, cbw *usb.CBW) (csw *usb.CSW, data []byte, err error) {
	op := cmd[0]
	length := int(cbw.DataTransferLength)

	// p8, 3.3 Host/Device Packet Transfer Order, USB Mass Storage Class 1.0
	csw = &usb.CSW{Tag: cbw.Tag}
	csw.SetDefaults()

	lun := int(cbw.LUN)

	if int(lun+1) > len(cards) {
		err = fmt.Errorf("invalid LUN")
		return
	}

	card := cards[lun]

	switch op {
	case INQUIRY:
		data = inquiry(length)
	case REQUEST_SENSE:
		data, err = sense(length)
	case MODE_SENSE_6, MODE_SENSE_10:
		data, err = modeSense(length)
	case REPORT_LUNS:
		data, err = reportLUNs(length)
	case READ_FORMAT_CAPACITIES:
		data, err = readFormatCapacities(card)
	case READ_CAPACITY_10:
		data, err = readCapacity10(card)
	case READ_10, WRITE_10:
		lba := int(binary.BigEndian.Uint32(cmd[2:]))
		blocks := int(binary.BigEndian.Uint16(cmd[7:]))

		if op == READ_10 {
			err = read(card, lba, blocks)
		} else {
			size := int(cbw.DataTransferLength)

			if card.Info().BlockSize*blocks != size {
				err = fmt.Errorf("unexpected %d blocks write transfer length (%d)", blocks, size)
			}

			dataPending = &writeOp{
				csw:    csw,
				lun:    lun,
				lba:    lba,
				blocks: blocks,
				size:   size,
			}

			csw = nil
		}
	case SERVICE_ACTION:
		switch cmd[1] {
		case READ_CAPACITY_16:
			data, err = readCapacity16(card, length)
		default:
			err = fmt.Errorf("unsupported service action %#x %+v", op, cbw)
		}
	case TEST_UNIT_READY, PREVENT_ALLOW_MEDIUM_REMOVAL:
		// ignored events
	default:
		err = fmt.Errorf("unsupported CDB Operation Code %#x %+v", op, cbw)
	}

	return
}

func handleWrite(buf []byte) (err error) {
	if len(buf) != dataPending.size {
		return fmt.Errorf("len(buf) != size (%d != %d)", len(buf), dataPending.size)
	}

	return write(cards[dataPending.lun], dataPending.lba, buf)
}

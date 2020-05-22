// https://github.com/f-secure-foundry/armory-ums
//
// Copyright (c) F-Secure Corporation
// https://foundry.f-secure.com
//
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

package main

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/f-secure-foundry/tamago/imx6/usb"
)

// p65, 3. Direct Access Block commands (SPC-5 and SBC-4), SCSI Commands Reference Manual, Rev. J
const (
	TEST_UNIT_READY  = 0x00
	REQUEST_SENSE    = 0x03
	INQUIRY          = 0x12
	MODE_SENSE_6     = 0x1a
	MODE_SENSE_10    = 0x5a
	READ_CAPACITY_10 = 0x25
	READ_10          = 0x28
	WRITE_10         = 0x2a

	// 04-349r1 SPC-3 MMC-5 Merge PREVENT ALLOW MEDIUM REMOVAL commands
	PREVENT_ALLOW_MEDIUM_REMOVAL = 0x1e

	// p376, Table 359 Mode page codes and subpage codes, SCSI Commands Reference Manual, Rev. J
	PAGE_CODE_ALL = 0x3f

	INQUIRY_DATA_LENGTH = 36
	SENSE_DATA_LENGTH   = 18
)

const (
	// exactly 8 bytes required
	VendorID = "F-Secure"

	// exactly 8 bytes required
	ProductID = "UA-MK-II"
)

type writeOp struct {
	csw    *usb.CSW
	lun    int
	lba    int
	blocks int
	size   int
}

// buffer for write commands (which spawn across multiple USB transfers)
var dataPending *writeOp

// p94, 3.6.2 Standard INQUIRY data, SCSI Commands Reference Manual, Rev. J
func inquiry() (data []byte) {
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
	data[4] = 31

	// unused or obsolete flags
	data = append(data, make([]byte, 3)...)

	// vendor identification
	data = append(data, []byte(VendorID)...)
	// product identification
	data = append(data, []byte(ProductID)...)

	// remaining data
	data = append(data, make([]byte, INQUIRY_DATA_LENGTH-len(data))...)

	return
}

// p56, 2.4.1.2 Fixed format sense data, SCSI Commands Reference Manual, Rev. J
func sense() (data []byte) {
	data = make([]byte, SENSE_DATA_LENGTH)

	// error code
	data[0] = 0x70
	// no specific sense key
	data[2] = 0x00
	// additional sense length
	data[7] = byte(len(data) - 1 - 7)
	// no additional sense code
	data[12] = 0x00
	// no additional sense qualifier

	return
}

// p111, 3.11 MODE SENSE(6) command, SCSI Commands Reference Manual, Rev. J
func modeSense(pageCode byte) (res []byte, err error) {
	switch pageCode {
	case PAGE_CODE_ALL:
		// p378, 5.3.3 Mode parameter header formats, SCSI Commands Reference Manual, Rev. J
		// empty 8-byte response
		res = make([]byte, 8)
	default:
		return nil, fmt.Errorf("unsupported mode page code %#x", pageCode)
	}

	return
}

// p37, 2.1 Command Descriptor Block (CDB), SCSI Commands Reference Manual, Rev. J
type CDB6 struct {
	OperationCode uint8
}

// p155, 3.22 READ CAPACITY (10) command, SCSI Commands Reference Manual, Rev. J
func readCapacity(lun int) (res []byte, err error) {
	info := cards[lun].Info()

	if err != nil {
		return
	}

	buf := new(bytes.Buffer)
	binary.Write(buf, binary.BigEndian, uint32(info.Blocks))
	binary.Write(buf, binary.BigEndian, uint32(info.BlockSize))
	res = buf.Bytes()

	return
}

func read(lun int, lba int, blocks int) (err error) {
	data, err := cards[lun].ReadBlocks(lba, blocks)

	if err != nil {
		return err
	}

	queue <- data

	return
}

func write(lun int, lba int, buf []byte) (err error) {
	return cards[lun].WriteBlocks(lba, buf)
}

func handleCDB(cmd [16]byte, cbw *usb.CBW) (csw *usb.CSW, data []byte, next int, err error) {
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

	switch op {
	case TEST_UNIT_READY:
		return
	case INQUIRY:
		data = inquiry()

		if length > len(data) {
			err = fmt.Errorf("unsupported INQUIRY transfer length %d > %d", length, len(data))
		}
	case REQUEST_SENSE:
		data = sense()

		if length > len(data) {
			err = fmt.Errorf("unsupported REQUEST_SENSE transfer length %d > %d", length, len(data))
		}
	case MODE_SENSE_6, MODE_SENSE_10:
		data, err = modeSense(cmd[2])
	case READ_CAPACITY_10:
		data, err = readCapacity(lun)
	case READ_10, WRITE_10:
		lba := int(binary.BigEndian.Uint32(cmd[2:]))
		blocks := int(binary.BigEndian.Uint16(cmd[7:]))

		if op == READ_10 {
			err = read(lun, lba, blocks)
		} else {
			next = int(cbw.DataTransferLength)

			if cards[lun].Info().BlockSize*blocks != next {
				err = fmt.Errorf("unexpected %d blocks write transfer length (%d)", blocks, int(cbw.DataTransferLength))
			}

			dataPending = &writeOp{
				csw:    csw,
				lun:    lun,
				lba:    lba,
				blocks: blocks,
				size:   next,
			}

			csw = nil
		}
	case PREVENT_ALLOW_MEDIUM_REMOVAL:
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

	return write(dataPending.lun, dataPending.lba, buf)
}

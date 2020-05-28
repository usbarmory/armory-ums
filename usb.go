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
	"errors"
	"fmt"

	"github.com/f-secure-foundry/tamago/imx6/usb"
)

const maxPacketSize = 512

// queue for device responses (IN)
var queue = make(chan []byte, 1024)

func configureDevice(device *usb.Device) {
	// Supported Language Code Zero: English
	device.SetLanguageCodes([]uint16{0x0409})

	// device descriptor
	device.Descriptor = &usb.DeviceDescriptor{}
	device.Descriptor.SetDefaults()

	// http://pid.codes/1209/2702/
	device.Descriptor.VendorId = 0x1209
	device.Descriptor.ProductId = 0x2702

	device.Descriptor.Device = 0x0001

	iManufacturer, _ := device.AddString(`TamaGo`)
	device.Descriptor.Manufacturer = iManufacturer

	iProduct, _ := device.AddString(`Storage Media`)
	device.Descriptor.Product = iProduct

	iSerial, _ := device.AddString(`0.1`)
	device.Descriptor.SerialNumber = iSerial

	conf := &usb.ConfigurationDescriptor{}
	conf.SetDefaults()

	device.AddConfiguration(conf)

	// device qualifier
	device.Qualifier = &usb.DeviceQualifierDescriptor{}
	device.Qualifier.SetDefaults()
	device.Qualifier.NumConfigurations = uint8(len(device.Configurations))
}

func buildMassStorageInterface() (iface *usb.InterfaceDescriptor) {
	// interface
	iface = &usb.InterfaceDescriptor{}
	iface.SetDefaults()
	iface.NumEndpoints = 2
	// Mass Storage
	iface.InterfaceClass = 0x8
	// SCSI
	iface.InterfaceSubClass = 0x6
	// Bulk-Only
	iface.InterfaceProtocol = 0x50
	iface.Interface = 0

	// EP1 IN endpoint (bulk)
	ep1IN := &usb.EndpointDescriptor{}
	ep1IN.SetDefaults()
	ep1IN.EndpointAddress = 0x81
	ep1IN.Attributes = 2
	ep1IN.MaxPacketSize = maxPacketSize
	ep1IN.Zero = false
	ep1IN.Function = tx

	iface.Endpoints = append(iface.Endpoints, ep1IN)

	// EP2 OUT endpoint (bulk)
	ep1OUT := &usb.EndpointDescriptor{}
	ep1OUT.SetDefaults()
	ep1OUT.EndpointAddress = 0x01
	ep1OUT.Attributes = 2
	ep1OUT.MaxPacketSize = maxPacketSize
	ep1OUT.Zero = false
	ep1OUT.Function = rx

	iface.Endpoints = append(iface.Endpoints, ep1OUT)

	return
}

// setup handles the class specific control requests specified at
// p7, 3.1 - 3.2, USB Mass Storage Class 1.0
func setup(setup *usb.SetupData) (in []byte, err error) {
	if setup == nil {
		return
	}

	switch setup.Request {
	case usb.BULK_ONLY_MASS_STORAGE_RESET:
		// For we ack this request without resetting.
	case usb.GET_MAX_LUN:
		if len(cards) == 0 {
			return nil, errors.New("unsupported")
		}

		in = []byte{byte(len(cards) - 1)}
	default:
		err = fmt.Errorf("unsupported request code: %#x", setup.Request)
	}

	return
}

func tx(_ []byte, lastErr error) (in []byte, err error) {
	select {
	case res := <-queue:
		in = res
	default:
	}

	return
}

func parseCBW(buf []byte) (cbw *usb.CBW, err error) {
	if len(buf) == 0 {
		return
	}

	if len(buf) != usb.CBW_LENGTH {
		return nil, fmt.Errorf("invalid CBW size %d != %d", len(buf), usb.CBW_LENGTH)
	}

	cbw = &usb.CBW{}
	err = binary.Read(bytes.NewReader(buf), binary.LittleEndian, cbw)

	if err != nil {
		return
	}

	if cbw.Length < 6 || cbw.Length > usb.CBW_CB_MAX_LENGTH {
		return nil, fmt.Errorf("invalid Command Block Length %d", cbw.Length)
	}

	if cbw.Signature != usb.CBW_SIGNATURE {
		return nil, fmt.Errorf("invalid CBW signature %x", cbw.Signature)
	}

	return
}

func rx(buf []byte, lastErr error) (res []byte, err error) {
	var cbw *usb.CBW

	if dataPending != nil {
		err = handleWrite(buf[0:dataPending.size])

		if err != nil {
			return
		}

		csw := dataPending.csw
		csw.DataResidue = 0

		queue <- dataPending.csw.Bytes()
		dataPending = nil

		return
	}

	cbw, err = parseCBW(buf)

	if err != nil {
		return
	}

	csw, data, next, err := handleCDB(cbw.CommandBlock, cbw)

	defer func() {
		if csw != nil {
			queue <- csw.Bytes()
		}
	}()

	if err != nil {
		csw.DataResidue = cbw.DataTransferLength
		csw.Status = usb.CSW_STATUS_COMMAND_FAILED
		return
	}

	if len(data) > 0 {
		queue <- data
	}

	if next != 0 {
		res = make([]byte, next)
	}

	return
}

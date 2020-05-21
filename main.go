// https://github.com/f-secure-foundry/armory-ums
//
// Copyright (c) F-Secure Corporation
// https://foundry.f-secure.com
//
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

package main

import (
	"fmt"
	"log"

	"github.com/f-secure-foundry/tamago/imx6"
	"github.com/f-secure-foundry/tamago/imx6/usb"

	"github.com/f-secure-foundry/tamago/usbarmory/mark-two"
)

func init() {
	log.SetFlags(0)

	if !imx6.Native {
		return
	}

	if err := imx6.SetARMFreq(900); err != nil {
		panic(fmt.Sprintf("WARNING: error setting ARM frequency: %v\n", err))
	}
}

// TODO: multi-LUN support to expose eMMC as well
var drive = usbarmory.SD

func main() {
	if err := drive.Detect(); err != nil {
		log.Printf("imx6_usdhc: SD card error, %v", err)
		return
	}

	info := drive.Info()
	capacity := int64(info.BlockSize) * int64(info.Blocks)
	mebi := capacity / (1000 * 1000 * 1000)
	mega := capacity / (1024 * 1024 * 1024)

	log.Printf("imx6_usdhc: %d GB/%d GiB SD card detected %+v", mebi, mega, info)

	device := &usb.Device{
		Setup: setup,
	}
	configureDevice(device)

	iface := buildMassStorageInterface()
	device.Configurations[0].AddInterface(iface)

	imx6.SetDMA(dmaStart, dmaSize)

	usb.USB1.Init()
	usb.USB1.DeviceMode()
	usb.USB1.Reset()

	// never returns
	usb.USB1.Start(device)
}

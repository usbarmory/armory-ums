Introduction
============

This [TamaGo](https://github.com/usbarmory/tamago) based unikernel
allows USB Mass Storage interfacing for the [USB armory Mk II](https://github.com/usbarmory/usbarmory/wiki)
internal eMMC card as well as any inserted external microSD card.

Binary releases
===============

Pre-compiled binary releases are available
[here](https://github.com/usbarmory/armory-ums/releases).

Compiling
=========

The [TamaGo compiler](https://github.com/usbarmory/tamago-go) is automatically
downloaded and compiled as a `go tool` by the `Makefile`.

Alternatively the `TAMAGO` environment variable can overridden to use the
[latest binary release](https://github.com/usbarmory/tamago-go/releases/latest):

```sh
wget https://github.com/usbarmory/tamago-go/archive/refs/tags/latest.zip
unzip latest.zip
cd tamago-go-latest/src && ./all.bash
cd ../bin && export TAMAGO=`pwd`/go
```

Build the `armory-ums.imx` application executable (note that on secure booted
units the `imx_signed` target should be used with the relevant
[`HAB_KEYS`](https://github.com/usbarmory/usbarmory/wiki/Secure-boot-(Mk-II)) set.


```
git clone https://github.com/usbarmory/armory-ums && cd armory-ums
make imx
```

Note that the command above embeds build and VCS information that is useful
for understanding the origin of a binary, but this prevents the same binary
from being reproducibly built elsewhere. To strip this information so that
the binary can be reproducibly built elsewhere:

```
REPRODUCIBLE=1 make imx
```

Executing
=========

The resulting `armory-ums.imx` file can be executed over USB using
[SDP](https://github.com/usbarmory/usbarmory/wiki/Boot-Modes-(Mk-II)#serial-download-protocol-sdp).

SDP mode requires boot switch configuration towards microSD without any card
inserted, however armory-ums detects microSD card only at startup. Therefore,
when starting with SDP, to expose the microSD over mass storage, follow this
procedure:

  1. Remove the microSD card on a powered off device.
  2. Set microSD boot mode switch.
  3. Plug the device on a USB port to power it up in SDP mode.
  4. Insert the microSD card.
  5. Launch `imx_usb armory-ums.imx`.

Alternatively, to expose the internal eMMC card, armory-ums can be
[flashed on any microSD](https://github.com/usbarmory/usbarmory/wiki/Boot-Modes-(Mk-II)#flashing-imx-native-images).

Operation
=========

Once running, the USB armory Mk II can be used like any standard USB drive,
exposing both internal eMMC card as well as the external microSD card (if
present).

| Card              | Evaluation order | LED status¹ |
|:-----------------:|------------------|-------------|
| external microSD  | 1st              | white       |
| internal eMMC     | 2nd              | blue        |

¹ LED on indicates successful detection

Authors
=======

Andrea Barisani  
andrea@inversepath.com  

License
=======

armory-ums | https://github.com/usbarmory/armory-ums  
Copyright (c) The armory-ums authors. All Rights Reserved.

These source files are distributed under the BSD-style license found in the
[LICENSE](https://github.com/usbarmory/armory-ums/blob/master/LICENSE) file.

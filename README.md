Introduction
============

This [TamaGo](https://github.com/f-secure-foundry/tamago) based unikernel
allows USB Mass Storage interfacing for the [USB armory Mk II](https://github.com/f-secure-foundry/usbarmory/wiki)
internal eMMC card as well as any inserted external uSD card.

Binary releases
===============

Pre-compiled binary releases are available
[here](https://github.com/f-secure-foundry/armory-ums).

Compiling
=========

Build the [TamaGo compiler](https://github.com/f-secure-foundry/tamago-go)
(or use the [latest binary release](https://github.com/f-secure-foundry/tamago-go/releases/latest)):

```
git clone https://github.com/f-secure-foundry/tamago-go -b tamago1.14.3
cd tamago-go/src && ./all.bash
cd ../bin && export TAMAGO=`pwd`/go
```

Build the `armory-ums.imx` application executable (note that on secure booted
units the `imx_signed` target should be used instead).

```
git clone https://github.com/f-secure-foundry/armory-ums && cd armory-ums
make CROSS_COMPILE=arm-none-eabi imx
```

The resulting `armory-ums.imx` file can be executed over USB using
[SDP ](https://github.com/f-secure-foundry/usbarmory/wiki/Boot-Modes-(Mk-II)#serial-download-protocol-sdp).

Operation
=========

Once running, the USB armory Mk II can be used like a standard USB drive,
exposing both the internal eMMC card as well as the external uSD card (if
present).

| Card          | Evaluation order | LED status¹ |
|:-------------:|------------------|-------------|
| external uSD  | 1st              | white       |
| internal eMMC | 2nd              | blue        |

¹ LED on indicates successful detection

Authors
=======

Andrea Barisani  
andrea.barisani@f-secure.com | andrea@inversepath.com  

License
=======

GoKey | https://github.com/f-secure-foundry/armory-ums  
Copyright (c) F-Secure Corporation

This program is free software: you can redistribute it and/or modify it under
the terms of the GNU General Public License as published by the Free Software
Foundation under version 3 of the License.

This program is distributed in the hope that it will be useful, but WITHOUT ANY
WARRANTY; without even the implied warranty of MERCHANTABILITY or FITNESS FOR A
PARTICULAR PURPOSE. See the GNU General Public License for more details.

See accompanying LICENSE file for full details.

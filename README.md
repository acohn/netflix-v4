# netflix-v4 

This small Go program exists to solve an annoying problem: Netflix blocks
Hurricane Electric's [tunnelbroker.net](https://tunnelbroker.net) tunnels, 
which have been used to circumvent copyright laws. However, Netflix's client 
will prefer to use a blocked IPv6 tunnel over a native IPv4 connection, and will 
then error out.

This program cannot be used to circumvent Netflix's region blocking. It simply
forces Netflix clients to use IPv4, which is hopefully not discriminated against.

## Installation on an EdgeRouter X 

I use an Ubiquiti EdgeRouter X as my home router, and have configured the DNS
forwarder (dnsmasq) on it to forward requests for netflix.com (and associated
domains) to this program, which acts as a filtering DNS proxy, removing any AAAA
records for Netflix domains. While this program could run on any always-available
machine on the local network, it's most convenient to run it directly on the 
EdgeRouter itself. (Don't run it on, eg. a VPS, because then Netflix's CDN might
not geolocate you correctly and you might get slower-loading content)

The Ubiquiti EdgeRouter X uses a little-endian MIPS CPU, which is one of the 
architectures targeted by recent versions of the Go compiler. To build a stripped
(smaller, which matters when you're on a device with 256 MB of storage) binary for 
the EdgeRouter X's MIPS architecture:

```bash 
$ go get -ud github.com/acohn/netflix-v4
$ GOOS=linux GOARCH=mipsle go build -ldflags "-s -w" github.com/acohn/netflix-v4
```
Once built, scp the binary onto your EdgeRouter, into the /config persistent
partition, and configure it to be started on boot. I use a script like this in the
/config/scripts/post-config.d/ directory, which is automatically run on boot:

```bash
#!/bin/bash

started='/tmp/nfx_started'

if [ -e $started ]; then
exit 0
fi

( sleep 30 && /usr/bin/sudo -u nobody -b /config/scripts/netflix-v4 ) &

touch $started
```

To configure your EdgeRouter to forward Netflix requests to this program, enter
configuration mode in the EdgeRouter CLI and add the following:

```
set service dns forwarding options 'server=/netflix.com/nflximg.com/nflxext.com/nflxvideo.com/127.0.0.1#5353'
```

## Inspiration and Acknowledgements ##
This is pretty much a straight Go port of [Chris Howie's Python implementation](https://github.com/cdhowie/netflix-no-ipv6-dns-proxy).
However, installing that script's dependencies directly on the EdgeRouter itself
takes up a significant chunk of the EdgeRouter's available storage, which prevented
me from upgrading the device firmware without uninstalling them. Additionally, the 
Python interpreter uses significantly more memory than this script.

This would have not been possible without Miek Gieben's excellent [Go DNS library](https://github.com/miekg/dns).

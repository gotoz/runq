vscp
====

Command `vscp` provides a `scp`-like utility for copying files over VM
sockets.  It is meant to show example usage of package `vsock`, but is
also useful in scenarios where a virtual machine does not have
networking configured, but VM sockets are available.

Usage
-----

`vscp` has two modes of operation: receiving and sending.

```
$ vscp -h
Usage of vscp:
  -c uint
        send only: context ID of the remote VM socket
  -p uint
        - receive: port ID to listen on (random port by default)
        - send: port ID to connect to
  -r    receive files from another instance of vscp
  -s    send files to another instance of vscp
  -v    enable verbose logging to stderr
```

For example, let's transfer the contents of `/proc/cpuinfo` from a
virtual machine to a hypervisor.

First, start a server on the hypervisor.  The following command will:
  - enable verbose logging
  - start `vscp` as a server to receive data
  - specify port 1024 as the server's listener port
  - use `cpuinfo.txt` as an output file

```
hv $ vscp -v -r -p 1024 cpuinfo.txt
2017/03/10 10:51:48 receive: creating file "cpuinfo.txt" for output
2017/03/10 10:51:48 receive: opening listener: 1024
2017/03/10 10:51:48 receive: listening: host(2):1024
# listening for client connection
```

Next, in the virtual machine, start a client to send a file to
the server on the hypervisor.  The following command will:
  - enable verbose logging
  - start `vscp` as a client to send data
  - specify context ID 2 (host process) as the server's context ID
  - specify port 1024 as the server's port
  - use `/proc/cpuinfo` as an input file

```
vm $ vscp -v -s -c 2 -p 1024 /proc/cpuinfo
2017/03/10 10:56:18 send: opening file "/proc/cpuinfo" for input
2017/03/10 10:56:18 send: dialing: 2.1024
2017/03/10 10:56:18 send: client: vm(3):1077
2017/03/10 10:56:18 send: server: host(2):1024
2017/03/10 10:56:18 send: sending data
2017/03/10 10:56:18 send: transfer complete
vm $
```

The transfer is now complete.  You should see more output in the
hypervisor's terminal, and the server process should have exited.

```
hv $ vscp -v -r -p 1024 cpuinfo.txt
2017/03/10 10:51:48 receive: creating file "cpuinfo.txt" for output
2017/03/10 10:51:48 receive: opening listener: 1024
2017/03/10 10:51:48 receive: listening: host(2):1024
# listening for client connection
2017/03/10 10:55:39 receive: server: host(2):1024
2017/03/10 10:55:39 receive: client: vm(3):1077
2017/03/10 10:55:39 receive: receiving data
2017/03/10 10:55:39 receive: transfer complete
hv $
```

To verify the transfer worked as intended, you can check the hash
of the file on both sides using a tool such as `md5sum`.

```
hv $ md5sum cpuinfo.txt
cda57941b11f0c82da425eec5d837c26  cpuinfo.txt
```
```
vm $ md5sum /proc/cpuinfo
cda57941b11f0c82da425eec5d837c26  /proc/cpuinfo
```

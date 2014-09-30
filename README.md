# lumberjack

o/~ I'm a lumberjack and I'm ok! I sleep when idle, then I ship logs all day! I parse your logs, I eat the JVM agent for lunch! o/~

## Changes from upstream version

This is a fork of Lumberjack (aka Logstash-Forwarder). It has a number of
enhancements over the upstream version:

* Establish multiple concurrent connections to upstream servers. If you specify
  more than one Logstash server, Lumberjack will connect to all of them and
  round-robin sends to each server.
* Exponential backoff on transport failure. If connectivity to the Logstash
  server is lost, Lumberjack will impose an exponential backoff between
  reconnection attempts, up to 10s.
* Multi-line support. Lumberjack now allows you to do multi-line joins before
  sending your logs to Logstash. This example looks for lines which don't start
  with a string that looks like a date, and joins them to the previous line:

```
  {
    "paths": [ "/var/log/your-multiline-log.log" ],
    "fields": { "type": "your_log" },
    "join": [
      {
        "not": "^\\d{4}-\\d{2}-\\d{2}",
        "with": "previous"
      }
    ]
  }
```

* Better log rotation handling. Lumberjack should catch some edge cases with
  copytruncate and other log rotation schemes. In order to support this, we are
  using the inotify library which MAY have broken support for non-Linux systems.
* Multiple threads and workers. In order to gain better concurrency, Lumberjack
  now uses multiple threads and workers.
* Logfile output and HUP support. You can now log to a dedicated file, rather
  than stdout or syslog. Sending Lumberjack a HUP causes it to close and re-open
  its file handles.
* Management port and replay functionality. Lumberjack listens on a TCP port
  (default: 42586) and allows you to re-read log files and send them to
  Logstash.
* State file handling. A few cases which caused the state file to get
  overwritten or not written to correctly have been fixed.
* An HTTP port which exposes expvar (http://golang.org/pkg/expvar/) data on
  memory use, and the state of files which are currently being followed.

### New requirements

In order to build and run Lumberjack you need Go v1.3.
This build has also only been tested on Linux. It may work on OSX but dependence
on `inotify` may prevent it running on other operating systems.

### Running Lumberjack with new options

The new options are:

* `-cmd-port`: Default 42586. The management port number.
* `-log-file`: Log file name.
* `-num-workers`: Default 2xCPU. If this becomes overwhelming, reduce this to a
  smaller number, eg 5 or 10.
* `-pid-file`: Default lumberjack.pid. PID file name.
* `-temp-dir`: Temp dir to store files. This needs to be on the same filesystem
  as your `-progress-file`.
* `-threads`: Default 1. The number of OS threads to run.
* `-http`: A port to listen on to expose the internal state of the process,
  including memory states and the position of files which are being followed.

Example:
```
/opt/lumberjack/bin/lumberjack -config /etc/lumberjack.conf \
        -spool-size 500 \
        -from-beginning=true -threads 8 -num-workers 8 \
        -progress-file /var/run/.lumberjack -pid-file /var/run/lumberjack.pid \
        -log-file /var/log/lumberjack.log -temp-dir /var/run
```

### Replaying existing log files

```
nc localhost 42586
replay
usage: replay [filename] [--offset=N] [field1=value1 field2=value2 ... fieldN=valueN]
replay /path/to/file.log type=awesome_log
```

No further output is provided here, you can look at the output log to see
Lumberjack tailing your file.

## Questions and support

If you have questions and cannot find answers, please join the #logstash irc
channel on freenode irc or ask on the logstash-users@googlegroups.com mailing
list.

## What is this?

A tool to collect logs locally in preparation for processing elsewhere!

### Resource Usage Concerns

Perceived Problems: Some users view logstash releases as "large" or have a generalized fear of Java.

Actual Problems: Logstash, for right now, runs with a footprint that is not
friendly to underprovisioned systems such as EC2 micro instances; on other
systems it is fine. Lumberjack will exist until that is resolved.

### Transport Problems

Few log transport mechanisms provide security, low latency, and reliability.

lumberjack exists to provide a network protocol for transmission that is
secure, low latency, low resource usage, and reliable.

## Configuring

lumberjack is configured with a json file you specify with the -config flag:

`lumberjack -config yourstuff.json`

Here's a sample, with comments in-line to describe the settings. Please please
please keep in mind that comments are technically invalid in JSON, so you can't
include them in your config.:

    {
      # The network section covers network configuration :)
      "network": {
        # A list of downstream servers listening for our messages.
        # lumberjack will pick one at random and only switch if
        # the selected one appears to be dead or unresponsive
        "servers": [ "localhost:5043" ],

        # The path to your client ssl certificate (optional)
        "ssl certificate": "./lumberjack.crt",
        # The path to your client ssl key (optional)
        "ssl key": "./lumberjack.key",

        # The path to your trusted ssl CA file. This is used
        # to authenticate your downstream server.
        "ssl ca": "./lumberjack_ca.crt",

        # Network timeout in seconds. This is most important for lumberjack
        # determining whether to stop waiting for an acknowledgement from the
        # downstream server. If an timeout is reached, lumberjack will assume
        # the connection or server is bad and will connect to a server chosen
        # at random from the servers list.
        "timeout": 15
      },

      # The list of files configurations
      "files": [
        # An array of hashes. Each hash tells what paths to watch and
        # what fields to annotate on events from those paths.
        {
          "paths": [ 
            # single paths are fine
            "/var/log/messages",
            # globs are fine too, they will be periodically evaluated
            # to see if any new files match the wildcard.
            "/var/log/*.log"
          ],

          # A dictionary of fields to annotate on each event.
          "fields": { "type": "syslog" }
        }, {
          # A path of "-" means stdin.
          "paths": [ "-" ],
          "fields": { "type": "stdin" }
        }, {
          "paths": [
            "/var/log/apache/httpd-*.log"
          ],
          "fields": { "type": "apache" }
        }
      ]
    }

### Goals

* Minimize resource usage where possible (CPU, memory, network).
* Secure transmission of logs.
* Configurable event data.
* Easy to deploy with minimal moving parts.
* Simple inputs only:
  * Follows files and respects rename/truncation conditions.
  * Accepts `STDIN`, useful for things like `varnishlog | lumberjack...`.

## Building it

1. Install [FPM](https://github.com/jordansissel/fpm)

        $ sudo gem install fpm

2. Install [go](http://golang.org/doc/install)


3. Compile lumberjack

        $ git clone git://github.com/jordansissel/lumberjack.git
        $ cd lumberback
        $ go build

4. Make packages, either:

        $ make rpm

    Or:

        $ make deb

## Installing it (via packages only)

If you don't use rpm or deb make targets as above, you can skip this section.

Packages install to `/opt/lumberjack`. Lumberjack builds all necessary
dependencies itself, so there should be no run-time dependencies you
need.

## Running it

Generally:

    $ lumberjack.sh -config lumberjack.conf

See `lumberjack.sh -help` for all the flags

The config file is documented further up in this file.

### Key points

* You'll need an SSL CA to verify the server (host) with.
* You can specify custom fields for each set of paths in the config file. Any
  number of these may be specified. I use them to set fields like `type` and
  other custom attributes relevant to each log.

### Generating an ssl certificate

Logstash supports all certificates, including self-signed certificates. To generate a certificate, you can run the following command:

    $ openssl req -x509 -batch -nodes -newkey rsa:2048 -keyout lumberjack.key -out lumberjack.crt

This will generate a key at `lumberjack.key` and the certificate at `lumberjack.crt`. Both the server that is running lumberjack as well as the logstash instances receiving logs will require these files on disk to verify the authenticity of messages.

Recommended file locations:

- certificates: `/etc/pki/tls/certs`
- keys: `/etc/pki/tls/private`

## Use with logstash

In logstash, you'll want to use the [lumberjack](http://logstash.net/docs/latest/inputs/lumberjack) input, something like:

    input {
      lumberjack {
        # The port to listen on
        port => 12345

        # The paths to your ssl cert and key
        ssl_certificate => "path/to/ssl.crt"
        ssl_key => "path/to/ssl.key"

        # Set this to whatever you want.
        type => "somelogs"
      }
    }

## Implementation details 

Below is valid as of 2012/09/19

### Minimize resource usage

* Sets small resource limits (memory, open files) on start up based on the
  number of files being watched.
* CPU: sleeps when there is nothing to do.
* Network/CPU: sleeps if there is a network failure.
* Network: uses zlib for compression.

### Secure transmission

* Uses OpenSSL to verify the server certificates (so you know who you
  are sending to).
* Uses OpenSSL to transport logs.

### Configurable event data

* The protocol lumberjack uses supports sending a `string:string` map.

### Easy deployment

* The `make deb` or `make rpm` commands will package everything into a
  single DEB or RPM.

### Future protocol discussion

I would love to not have a custom protocol, but nothing I've found implements
what I need, which is: encrypted, trusted, compressed, latency-resilient, and
reliable transport of events.

* Redis development refuses to accept encryption support, would likely reject
  compression as well.
* ZeroMQ lacks authentication, encryption, and compression.
* Thrift also lacks authentication, encryption, and compression, and also is an
  RPC framework, not a streaming system.
* Websockets don't do authentication or compression, but support encrypted
  channels with SSL. Websockets also require XORing the entire payload of all
  messages - wasted energy.
* SPDY is still changing too frequently and is also RPC. Streaming requires
  custom framing.
* HTTP is RPC and very high overhead for small events (uncompressable headers,
  etc). Streaming requires custom framing.

## License 

See LICENSE file.


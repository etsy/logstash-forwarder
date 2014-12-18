# lumberjack

o/~ I'm a lumberjack and I'm ok! I sleep when idle, then I ship logs all day! I parse your logs, I eat the JVM agent for lunch! o/~

## Changes from upstream version

This is a fork of Lumberjack (aka Logstash-Forwarder). It has a number of
enhancements over the upstream version:

* Establish multiple concurrent connections to upstream servers. If you specify
  more than one Logstash server, Lumberjack will connect to all of them and
  distribute where log lines are sent. If one server becomes busy and responds
  slowly, that worker will block while others continue to send data to other
  servers.
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
* Multiple threads. In order to gain better concurrency, Lumberjack now uses
  multiple threads.
* Logfile output and HUP support. You can now log to a dedicated file, rather
  than stdout or syslog. Sending Lumberjack a HUP causes it to close and re-open
  its own log file.
* State file handling. A few cases which caused the state file to get
  overwritten or not written to correctly have been fixed.
* An HTTP port which exposes expvar (http://golang.org/pkg/expvar/) data on
  memory use, and the state of files which are currently being followed.
* Sending logs to different logstash servers. Lumberjack now supports multiple
  destinations. This allows you to send logs to different Logstash clusters. The
  configuration file with this feature looks like:

```
{
    "files": [
        {
            "paths": ["/var/log/httpd/access_log"],
            "fields": { "type": "access_log" }
        }, 
        {
            "paths": ["/var/log/httpd/error_log"],
            "fields": { "type": "error_log" },
            "dest": "one"
        },
        {
            "paths": ["/var/log/app.log"],
            "fields": { "type": "app_log" },
            "dest": "two"
        }
    ],
    "network": [
        {
            "servers": [ "logstash-default:9991"],
            "ssl ca": "/etc/pki/logstash/lumberjack.crt"
        },
        {
            "name": "one",
            "servers": ["logstash-one:9991"],
            "ssl ca": "/etc/pki/logstash/lumberjack.crt"
        },
        {
            "name": "two",
            "servers": ["logstash-two:9991"],
            "ssl ca": "/etc/pki/logstash/lumberjack.crt"
        }
    ]
}
```

Files take an optional `dest` parameter, which corresponds to the name in the
`network` section.
One of the groups of servers under the `network` section should be left without
a name - this will be given the name `default`. Any files which do not have a
destination, will be sent to the default servers.

### New requirements

In order to build and run Lumberjack you need Go v1.3.
This build has also only been tested on Linux. It may work on OSX but dependence
on `inotify` may prevent it running on other operating systems.

### Running Lumberjack with new options

The new options are:

* `-cmd-port`: Default 42586. The management port number.
* `-log-file`: Log file name.
* `-pid-file`: Default lumberjack.pid. PID file name.
* `-temp-dir`: Temp dir to store files. This needs to be on the same filesystem
  as your `-progress-file`.
* `-threads`: Default 2xCPU. The number of OS threads to run.
* `-http`: A port to listen on to expose the internal state of the process,
  including memory states and the position of files which are being followed.

Example:
```
/opt/lumberjack/bin/lumberjack -config /etc/lumberjack.conf \
        -spool-size 500 \
        -from-beginning=true -threads 8 \
        -progress-file /var/run/.lumberjack -pid-file /var/run/lumberjack.pid \
        -log-file /var/log/lumberjack.log -temp-dir /var/run
```

### Getting expvar data

Start Lumberjack with the `-http` option, with a port number. Assuming the port
number is 9999, run the following (output formatted for clarity):

```
$ curl localhost:9999/debug/vars

{
    "cmdline":
    ["/opt/lumberjack/bin/lumberjack","--config","/etc/lumberjack_random.conf","--http",":9999","--cmd-port","42587"],
    "memstats":
    {
        "Alloc":890760,
        "TotalAlloc":7938976,
        "Sys":8919288,
        "Lookups":497,
        "Mallocs":25199,
        "Frees":22621,
        "HeapAlloc":890760,
        "HeapSys":5242880,
        "HeapIdle":3997696,
        "HeapInuse":1245184,
        "HeapReleased":3284992,
        "HeapObjects":2578,
        "StackInuse":155648,
        "StackSys":524288,
        "MSpanInuse":14336,
        "MSpanSys":32768,
        "MCacheInuse":2200,
        "MCacheSys":16384,
        "BuckHashSys":1443904,
        "GCSys":1380352,
        "OtherSys":278712,
        "NextGC":1515312,
        "LastGC":1412115488192038181,
        "PauseTotalNs":5269967,
        "PauseNs":[176775,128580,351280,284971,552778,330809,717339,711930,655566,910777,449162,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0],
        "NumGC":11,
        "EnableGC":true,
        "DebugGC":false,
        "BySize":[{"Size":0,"Mallocs":0,"Frees":0},
            {"Size":8,"Mallocs":702,"Frees":675},
            {"Size":16,"Mallocs":6509,"Frees":6076},
            {"Size":32,"Mallocs":3663,"Frees":3374},
            {"Size":48,"Mallocs":2984,"Frees":2797},
            {"Size":64,"Mallocs":314,"Frees":273},
            {"Size":80,"Mallocs":3019,"Frees":2390},
            {"Size":96,"Mallocs":537,"Frees":511},
            {"Size":112,"Mallocs":381,"Frees":342},
            {"Size":128,"Mallocs":394,"Frees":377},
            {"Size":144,"Mallocs":2990,"Frees":2467},
            {"Size":160,"Mallocs":227,"Frees":205},
            {"Size":176,"Mallocs":477,"Frees":460},
            {"Size":192,"Mallocs":404,"Frees":381},
            {"Size":208,"Mallocs":502,"Frees":435},
            {"Size":224,"Mallocs":37,"Frees":17},
            {"Size":240,"Mallocs":15,"Frees":13},
            {"Size":256,"Mallocs":11,"Frees":10},
            {"Size":288,"Mallocs":477,"Frees":409},
            {"Size":320,"Mallocs":26,"Frees":16},
            {"Size":352,"Mallocs":296,"Frees":280},
            {"Size":384,"Mallocs":0,"Frees":0},
            {"Size":416,"Mallocs":32,"Frees":20},
            {"Size":448,"Mallocs":190,"Frees":185},
            {"Size":480,"Mallocs":1,"Frees":0},
            {"Size":512,"Mallocs":12,"Frees":11},
            {"Size":576,"Mallocs":113,"Frees":108},
            {"Size":640,"Mallocs":4,"Frees":0},
            {"Size":704,"Mallocs":9,"Frees":4},
            {"Size":768,"Mallocs":3,"Frees":3},
            {"Size":896,"Mallocs":24,"Frees":13},
            {"Size":1024,"Mallocs":98,"Frees":91},
            {"Size":1152,"Mallocs":110,"Frees":94},
            {"Size":1280,"Mallocs":1,"Frees":1},
            {"Size":1408,"Mallocs":5,"Frees":1},
            {"Size":1536,"Mallocs":10,"Frees":10},
            {"Size":1664,"Mallocs":15,"Frees":11},
            {"Size":2048,"Mallocs":6,"Frees":3},
            {"Size":2304,"Mallocs":97,"Frees":93},
            {"Size":2560,"Mallocs":0,"Frees":0},
            {"Size":2816,"Mallocs":0,"Frees":0},
            {"Size":3072,"Mallocs":0,"Frees":0},
            {"Size":3328,"Mallocs":111,"Frees":108},
            {"Size":4096,"Mallocs":264,"Frees":251},
            {"Size":4608,"Mallocs":89,"Frees":89},
            {"Size":5376,"Mallocs":1,"Frees":0},
            {"Size":6144,"Mallocs":0,"Frees":0},
            {"Size":6400,"Mallocs":2,"Frees":0},
            {"Size":6656,"Mallocs":0,"Frees":0},
            {"Size":6912,"Mallocs":0,"Frees":0},
            {"Size":8192,"Mallocs":16,"Frees":6},
            {"Size":8448,"Mallocs":0,"Frees":0},
            {"Size":8704,"Mallocs":0,"Frees":0},
            {"Size":9472,"Mallocs":0,"Frees":0},
            {"Size":10496,"Mallocs":0,"Frees":0},
            {"Size":12288,"Mallocs":0,"Frees":0},
            {"Size":13568,"Mallocs":0,"Frees":0},
            {"Size":14080,"Mallocs":0,"Frees":0},
            {"Size":16384,"Mallocs":1,"Frees":1},
            {"Size":16640,"Mallocs":0,"Frees":0},
            {"Size":17664,"Mallocs":0,"Frees":0}]},
    "tailing":
    [{"path":"/var/log/httpd/access_log","id":"2195172_64512","fields":{"rotated":"false","type":"access_log"}},{"path":"/var/log/httpd/error_log","id":"2195110_64512","fields":{"type":"error_log"}}]

}
```

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


This directory contains a [dockerized](https://www.docker.com/whatisdocker/) version of `glcd`.

A `Makefile` is provided to simplify the process. Due to the way that `glcd` currently handles ip address configuration (in a config file), some configuration is needed. Fortunately this is fairly straightforward.

First, [https://docs.docker.com/installation/](install) `docker`.

Run `ip addr show docker0 | grep -Po 'inet \K[\d.]+'` to determine the `docker0` ip4 address.

Edit [glcd.config.default](../glcd.config.default) and set `nsqd-address` and `lookupd-address` to this value.

Run `make deps` to pull in the necessary `docker` instances. This step is not strictly necessary as `docker run` will automatically pull the relevant image if it is not found; however, if this step fails then something went wrong with the `docker` installation.

Once the images are downloaded, `make run` to run all docker instances. If all works, `glcd` will be running inside the docker virtual interface.

`make stop` will stop *all* `docker` instances. `make kill` will similarly remove **all** docker instances.

If you wish to build a `glcd` `docker` instance yourself, you may try `make build` and `make run-local`.

Note that `docker` support is still experimental; no guarantees that the above works!

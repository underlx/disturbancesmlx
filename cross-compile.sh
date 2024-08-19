#!/bin/sh
gox -osarch="linux/amd64" -tags="release" -ldflags="$(govvv -flags) -checklinkname=0" -output="disturbancesmlx"
#!/bin/bash

exec /usr/bin/ganesha.nfsd -F -L /dev/stderr -f /etc/ganesha/ganesha.conf

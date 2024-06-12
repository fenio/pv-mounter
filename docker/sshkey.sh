#!/bin/bash
/usr/bin/xargs -0 -L1 -a /proc/1/environ | /usr/bin/sed -n 's/SSH_PUBLIC_KEY=//gp'

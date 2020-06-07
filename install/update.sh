#!/bin/bash

systemctl disable vert
systemctl stop vert
./install.sh
systemctl start vert
systemctl enable vert
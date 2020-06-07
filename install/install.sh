#!/bin/bash

INSTALL_DIR=/usr/local/vert

mkdir -p ${INSTALL_DIR}
mkdir -p ${INSTALL_DIR}/certs
mkdir -p /tmp/vert

chmod 700 ${INSTALL_DIR}/certs

cp Vert ${INSTALL_DIR}/Vert
cp conf.yaml ${INSTALL_DIR}/conf.yaml
cp vert.service /etc/systemd/system/vert.service
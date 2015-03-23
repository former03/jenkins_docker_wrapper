#!/bin/bash

NAME="jenkins_docker_wrapper"
DEST_PATH="/usr/bin/${NAME}"

for host in work1.dmz work2.dmz; do
    scp $NAME "root@${host}:${DEST_PATH}"
    ssh "root@${host}" "chown root:docker ${DEST_PATH}; chmod 4755 ${DEST_PATH}"
done

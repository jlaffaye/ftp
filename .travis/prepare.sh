#!/bin/sh -e
sudo apt-get update
mkdir --mode 0777 -p /var/ftp/incoming

case "$1" in
  proftpd)
    mkdir -p /etc/proftpd/conf.d/
    cp $TRAVIS_BUILD_DIR/.travis/proftpd.conf /etc/proftpd/conf.d/
    apt-get install -qq "$1"
    ;;
  vsftpd)
    cp $TRAVIS_BUILD_DIR/.travis/vsftpd.conf /etc/vsftpd.conf
    apt-get install -qq "$1"
    ;;
  vsftpd_implicit_tls)
    openssl req \
    -new \
    -newkey rsa:1024 \
    -days 365 \
    -nodes \
    -x509 \
    -subj "/C=US/ST=Denial/L=Springfield/O=Dis/CN=localhost" \
    -keyout /etc/ssl/certs/vsftpd.pem \
    -out /etc/ssl/certs/vsftpd.pem
    cp $TRAVIS_BUILD_DIR/.travis/vsftpd_implicit_tls.conf /etc/vsftpd.conf
    apt-get install -qq vsftpd
    ;;
  *)
    echo "unknown software: $1"
    exit 1
esac

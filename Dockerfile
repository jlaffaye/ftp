FROM golang:1.6.0

RUN mkdir --mode 0777 -p /var/ftp/incoming
RUN mkdir -p /var/run/vsftpd/empty
RUN mkdir -p $GOPATH/src/github.com/jlaffaye/ftp

WORKDIR $GOPATH/src/github.com/jlaffaye/ftp

RUN go get github.com/axw/gocov/gocov
RUN go get github.com/mattn/goveralls

RUN apt-get update && apt-get install -y --no-install-recommends vsftpd && apt-get clean

RUN echo "local_enable=YES" >> /etc/vsftpd.conf && \
  echo "anon_root=/var/ftp" >> /etc/vsftpd.conf && \
  echo "anon_upload_enable=YES" >> /etc/vsftpd.conf && \
  echo "anonymous_enable=YES" >> /etc/vsftpd.conf && \
  echo "dirmessage_enable=YES" >> /etc/vsftpd.conf && \
  echo "anon_umask=022" >> /etc/vsftpd.conf && \
  echo "anon_mkdir_write_enable=YES" >> /etc/vsftpd.conf && \
  echo "anon_other_write_enable=YES" >> /etc/vsftpd.conf && \
  echo "secure_chroot_dir=/var/run/vsftpd/empty" >> /etc/vsftpd.conf && \
  echo "listen_ipv6=YES" >> /etc/vsftpd.conf && \
  echo "chroot_local_user=YES" >> /etc/vsftpd.conf && \
  echo "allow_writeable_chroot=YES" >> /etc/vsftpd.conf && \
  echo "write_enable=YES" >> /etc/vsftpd.conf && \
  echo "background=YES" >> /etc/vsftpd.conf && \
  echo "local_umask=022" >> /etc/vsftpd.conf && \
  echo "#!/bin/sh" >> test.sh && \
  echo "vsftpd /etc/vsftpd.conf" >> test.sh && \
  echo "go test" >> test.sh && \
  chmod +x test.sh

COPY ./ .

CMD ["./test.sh"]

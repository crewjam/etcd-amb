FROM gliderlabs/alpine
RUN apk --update add haproxy ca-certificates
ADD etcd-amb /bin/etcd-amb
CMD etcd-amb
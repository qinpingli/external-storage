FROM fedora:22
LABEL authors="Ping Qin <piqin@redhat.com>" 

#install targetcli
RUN dnf install -y targetcli

# install iscsi privisioner
ADD targetd2 /iscsi-controller

ADD init.sh /
ENTRYPOINT ["/init.sh"]

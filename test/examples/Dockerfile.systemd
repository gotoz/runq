FROM ubuntu:18.04

RUN apt-get update \
    && apt-get install -y \
        iproute2 \
        iputils-ping \
        kmod \
        openssh-server \
        systemd \
        udev \
     && systemctl enable ssh.service

RUN echo "rootfs / none defaults,private  0  0" > /etc/fstab
RUN echo "PermitRootLogin yes" >> /etc/ssh/sshd_config
RUN echo "root:root" | chpasswd && passwd -u root


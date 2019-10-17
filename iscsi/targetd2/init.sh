#!/bin/bash

# enable dbus
mkdir /run/dbus
dbus-daemon --system

#targets=(`targetcli /iscsi ls|grep iqn|awk {'print $2'}`)
#for i in "${targets[@]}"
#do
#	targetcli /iscsi delete $i
#done

#disks=(`targetcli /backstores/fileio ls|grep iscsi_disks|awk {'print $2'}`)
#for i in "${disks[@]}"
#do
#	targetcli /backstores/fileio delete $i
#done

systemctl enable targetd.service
systemctl restart targetd.service

/iscsi-controller $*

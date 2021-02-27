#!/bin/bash

sudo useradd uberswitch -s /sbin/nologin -M -G gpio
sudo cp uberswitch.service /lib/systemd/system
sudo chmod 0755 /lib/systemd/system/uberswitch.service 
sudo systemctl enable uberswitch.service

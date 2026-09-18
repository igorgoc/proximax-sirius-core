name: "ProximaX Sirius Validator"
description: "Ultra-lightweight ProximaX Sirius Mainnet POS+ Peer Validator Node for Home Assistant (ARM64 / RPi4)"
version: "1.9.8"
slug: "proximax_sirius_validator"
arch:
  - aarch64
  - amd64
init: false
homeassistant_api: false
startup: services
boot: auto
ports:
  7900/tcp: 7900
ports_description:
  7900/tcp: "Sirius P2P Network Port for block synchronization & validator gossip"
options:
  boot_key: ""
  harvest_key: ""
  friendly_name: "HomeAssistant-Validator"
  fast_sync: true
  custom_snapshot_url: ""
schema:
  boot_key: "password"
  harvest_key: "password"
  friendly_name: "str"
  fast_sync: "bool"
  custom_snapshot_url: "str?"
environment:
  CHAINCONFIG_DIR: "/data/chainconfig"

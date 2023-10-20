#!/usr/bin/env bash

# integers
./your_bittorrent.sh decode i32e
./your_bittorrent.sh decode i10000e
./your_bittorrent.sh decode i-1000e
# strings
./your_bittorrent.sh decode 5:william
./your_bittorrent.sh decode 1:w
./your_bittorrent.sh decode 5:hello
# dictionaries
./your_bittorrent.sh decode de
./your_bittorrent.sh decode d10:inner_dictd4:keysi32eee
./your_bittorrent.sh decode d10:inner_dictd4:key16:value14:key2i42e8:list_keyl5:item15:item2i3eeee
# lists
./your_bittorrent.sh decode le
./your_bittorrent.sh decode l10:inner_dictd4:keysi32ee
./your_bittorrent.sh decode l5:item15:item2i3ee

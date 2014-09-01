#!/bin/sh
#
# Copyright 2014 The Mangos Authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use file except in compliance with the License.
# You may obtain a copy of the license at
#
#    http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

url0=tcp://127.0.0.1:40890
url1=tcp://127.0.0.1:40891
url2=tcp://127.0.0.1:40892
url3=tcp://127.0.0.1:40893
./bus node0 $url0 $url1 $url2 & node0=$!
./bus node1 $url1 $url2 $url3 & node1=$!
./bus node2 $url2 $url3 & node2=$!
./bus node3 $url3 $url0 & node3=$!
sleep 5
kill $node0 $node1 $node2 $node3


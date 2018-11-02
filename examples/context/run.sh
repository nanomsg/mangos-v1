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

url=tcp://127.0.0.1:40899

./context server $url & server=$! && sleep 1

# Start many client requests at the same time
# Note that there are more client requests than worker threads.
# As worker threads become free they start to service other requests
# in the queue.
./context client $url "John" & c1=$!
./context client $url "Bill" & c2=$!
./context client $url "Mary" & c3=$!
./context client $url "Susan" & c4=$!
./context client $url "Mark" & c5=$!

wait $c1 $c2 $c3 $c4 $c5

kill $server

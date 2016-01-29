#!/bin/bash

NUM=9
HOST=127.0.0.1
for i in `seq 0 $NUM`; do
  serf agent -node=serf$i -bind=$HOST:$((15000 + i)) -rpc-addr=$HOST:$((16000 + i)) -tag rphost=$HOST -tag rpport=$((17000 + i)) -event-handler ./eh.py &
  ../testpop --listen $HOST:$((17000 + i)) &
done
for i in `seq 1 $NUM`; do
  serf join -rpc-addr=$HOST:$((16000 + i)) $HOST:15000
  sleep 1
done

#killall serf

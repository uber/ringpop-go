#!/bin/sh

instance_no=$1

echo "running instance $instance_no"

BIND="127.0.0.1"

SERF="/usr/local/bin/serf"
RINGPOP="./testpop"

base_port=1300
instance_port=`expr 10 \* $instance_no`
instance_port=`expr $base_port + $instance_port`

serf_node="SERF-$instance_no"
serf_master="$BIND:`expr $base_port + 1`"
serf_bind="$BIND:`expr $instance_port + 1`"
serf_rpc_bind="$BIND:`expr $instance_port + 2`"
ringpop_port="`expr $instance_port + 3`"
ringpop_bind="$BIND:$ringpop_port"


echo "Starting serf on $serf_bind (rpc: $serf_rpc_bind)..."
$SERF agent -bind=$serf_bind -rpc-addr=$serf_rpc_bind -node=$serf_node -tag rphost=$BIND -tag rpport=$ringpop_port -event-handler=./serf_events/eh.py &

sleep 5
echo "Starting ringpop ($ringpop_bind)"
$RINGPOP -listen $ringpop_bind &

if [ $instance_no -ne 0 ]; then
	echo "Joining $serf_master"
	$SERF join -rpc-addr=$serf_rpc_bind $serf_master
fi


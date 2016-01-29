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
serf_pid=$!

echo "Starting ringpop ($ringpop_bind)"
$RINGPOP -listen $ringpop_bind &
ringpop_pid=$!

if [ $instance_no -ne 0 ]; then
	echo "Joining $serf_master"
	$SERF join -rpc-addr=$serf_rpc_bind $serf_master
fi

echo "serf_pid: $serf_pid, ringpop_pid: $ringpop_pid"
running=1

health_check() {
	if ! kill -0 $serf_pid > /dev/null 2>&1; then
		echo "serf down"
		return -1
	fi

	if ! kill -0 $ringpop_pid > /dev/null 2>&1; then
		echo "ringpop down"
		return -2
	fi

	return 0
}

trap_handler() {
	running=0
}

quit() {
	echo "Quiting..."
	kill $serf_pid > /dev/null 2>&1
	kill $ringpop_pid > /dev/null 2>&1
}

trap trap_handler INT

while health_check
do
	if [ $running -ne 1 ]; then
		break
	fi

	echo "Health check passed"
	sleep 1
done

quit

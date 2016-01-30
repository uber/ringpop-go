#!/bin/bash

instance_no=$1

e() {
	echo "$instance_no: $1"
}

e "running instance $instance_no"

BIND="127.0.0.1"

SERF="/usr/local/bin/serf"
RINGPOP="./ping-ring"

base_port=1300
instance_port=`expr 10 \* $instance_no`
instance_port=`expr $base_port + $instance_port`

serf_node="SERF-$instance_no"
serf_master="$BIND:`expr $base_port + 1`"
serf_bind="$BIND:`expr $instance_port + 1`"
serf_rpc_bind="$BIND:`expr $instance_port + 2`"
ringpop_port="`expr $instance_port + 3`"
ringpop_bind="$BIND:$ringpop_port"


e "Starting serf on $serf_bind (rpc: $serf_rpc_bind)..."
$SERF agent -bind=$serf_bind -rpc-addr=$serf_rpc_bind -node=$serf_node -tag rphost=$BIND -tag rpport=$ringpop_port &
serf_pid=$!

e "Starting ringpop ($ringpop_bind)"
$RINGPOP -listen $ringpop_bind &
ringpop_pid=$!

if [ $instance_no -ne 0 ]; then
	sleep 2
	e "Joining $serf_master"
	$SERF join -rpc-addr=$serf_rpc_bind $serf_master
fi

e "serf_pid: $serf_pid, ringpop_pid: $ringpop_pid"
running=1

health_check() {
	if ! kill -0 $serf_pid > /dev/null 2>&1; then
		e "serf down"
		return -1
	fi

	if ! kill -0 $ringpop_pid > /dev/null 2>&1; then
		e "ringpop down"
		return -2
	fi

	return 0
}

trap_handler() {
	running=0
}

quit() {
	e "Quiting..."
	if kill -0 $serf_pid > /dev/null 2>&1; then
		e "Stopping serf ($serf_rpc_bind)"
		if [ $running -eq 1 ]; then
			$SERF leave -rpc-addr=$serf_rpc_bind
		fi
		wait $serf_pid
	fi

	if kill -0 $ringpop_pid > /dev/null 2>&1; then	
		e "Killing $RINGPOP"
		kill $ringpop_pid > /dev/null 2>&1
	fi
}

trap trap_handler 2

while health_check
do
	if [ $running -ne 1 ]; then
		break
	fi

	e "Health check passed"
	sleep 1
done

quit

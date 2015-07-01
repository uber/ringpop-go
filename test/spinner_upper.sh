#!/bin/bash


hostpids=()
pids=()
hosts=()

# takes a hostname in $1 and sends back the associated pid
hostpidsGet() {
    for hp in "${hostpids[@]}"; do
        local k=${hp%%/*}
        local v=${hp#*/}
        if [[ $k == $1 ]]; then
            echo $(($v))
            return
        fi
    done

    echo -1
}

# adds $1/$2 to hostpids
hostpidsAdd() {
    hostpids+=("$1/$2")
}


hostpidsRemove() {
    echo "temp"
    # do a get
    # then do a delete
}

# kills all ringpops
quit() {
    # echo ${pids[@]}
    for pid in ${pids[@]}; do
        kill $pid
        # if [[ $? -ne 0 ]]; then
        #     echo "failed to kill process ${pid}"
        # fi
    done
}

# kills n ringpops where n is $1
killn() {
    for (( i = 0; i < $1; i++ )); do
        if [[ ${#pids[@]} == 0 ]]; then
            break # no more processes can be killed
        fi

        ind=$(($RANDOM % ${#pids[@]}))
        echo $ind
        break
    done
}

# writes the contents of $hosts to ./hosts.json
# TODO: make this less shitty (tail -n or something?)
writeHosts () {
    echo "[" > ./hosts.json # overwrites old
    for (( i = 0; i < ${#hosts[@]} - 1; i++ )); do
        echo "\t${hosts[i]}," >> ./hosts.json
    done
    echo "\t${hosts[i]}" >> ./hosts.json
    echo "]" >> ./hosts.json
}

# adds host given by $1 to ./hosts.json
addHost () {
    hosts+=($1)

    # write hosts file
    writeHosts
}

# removes host given be $1 from ./hosts.json
removeHost() {
    delete=($1)
    hosts=(${hosts[@]/$delete})

    # write hosts file
    writeHosts
}

for i in `seq 1 $1`; do
    port=$((3000 + i))
    hostport="127.0.0.1:$port"
    addHost $hostport

    ./mock_testpop/mock_testpop $hostport &
    # ./testpop $hostport & # start ringpop instance

    pids+=($!)                  # grap pid so we can kill it
    # hostpidsAdd $hostport $!    # add to host/pid map
    hostpids+=("$hostport/$!")
done

while [[ true ]]; do
    #statements
    read cmd opts
    case $cmd in
        q)
            quit
            echo "...exiting"
            break
            ;;

        k)
            num="^[0-9]+$"
            if ! [[ ${opts[0]} =~ $num ]]; then
                echo "error: expected int for second arg" >&2
                continue
            fi
            killn ${opts[0]}
            ;;

        r)
            removeHost ${opts[0]}
            ;;

        g)
            pid=$(hostpidsGet ${opts[0]})
            if [[ $(($pid)) == -1 ]]; then
                echo "not found"
                continue
            fi
            echo $pid
            ;;

        *)
            continue
    esac
done

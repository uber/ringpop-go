
```bash
# start minikube and get it working with docker
minikube start
eval $(minikube docker-env)

# lets inspect our kubernetes installation
kubectl get all
docker ps

# build application and containter
GOOS=linux go build
docker build -t ringpop-k8s .
docker images

# create the service before making the first pod to make sure it is responding
# to dns lookups
kubectl create -f manifests/ringpop-k8s-svc.yaml
kubectl get all

# start one pod
kubectl create -f manifests/ringpop-k8s-pod.yaml
kubectl get all
kubectl logs -f ringpop-k8s

# Lets inspect our node
kubectl get pods -o jsonpath='{.items[*].status.podIP}'
ringpop-admin top -r 1000 `<ip of pod>`

# make the node routable
minikube ip
route -n add 172.17.0.0/16 `minikube ip`
ringpop-admin top -r 1000 `<ip of pod>`

# starting more pods via a replication controller (aka look mom, no hands)
kubectl create -f manifests/ringpop-k8s-rc.yaml
ringpop-admin top -r 1000 `<ip of pod>`
kubectl logs -f ringpop-k8s

# deleting some pods
kubectl delete pods ringpop-k8s
kubectl delete pods ringpop-k8s-v0.6.0-...

# kill the first container running our app
docker ps | grep "ringpop-k8s" | grep -v "pause" | head -n 1 | cut -d ' ' -f 1 | xargs docker stop

# or lets kill all
/docker ps | grep ringpop-k8s | grep -v "pause" | cut -d ' ' -f 1 | xargs docker stop
# don't actually do this on minikube it makes the dns server unhappy :(

# upgrading to a newer version
kubectl rolling-update ringpop-k8s-v0.6.0 -f manifests/ringpop-k8s-rc2.yaml --update-period 10s
```

package ringpop

import (
	"errors"
	"strconv"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
)

// replica consts
const defaultNReplicas = 3
const defaultRReplicas = 1
const defaultWReplicas = 3

type replicator struct {
	ringpop   *Ringpop
	nReplicas int //no: of total replicas
	rValue    int //min no: of reads that must be successful
	wValue    int //min no: of writes that must be succesful
}

type replicaOpts struct {
	keys      []string
	req       *forwardReq
	rValue    int
	wValue    int
	nReplicas int
	timeout   time.Duration
}

type replicaResp struct {
	keys        []string
	destRespMap map[string]int
}

func newReplicator(ringpop *Ringpop, nReplicas int, rValue int, wValue int) *replicator {
	r := &replicator{
		ringpop:   ringpop,
		nReplicas: nReplicas | defaultNReplicas,
		rValue:    rValue | defaultRReplicas,
		wValue:    wValue | defaultWReplicas,
	}

	return r
}

func newReplicaOpts(keys []string, req *forwardReq, rValue int, wValue int, nReplicas int, timeout time.Duration) *replicaOpts {
	r := &replicaOpts{
		req:       req,
		rValue:    rValue,
		wValue:    wValue,
		nReplicas: nReplicas,
		timeout:   timeout,
	}

	r.keys = make([]string, len(keys))
	copy(r.keys, keys)
	return r
}
func groupReplicas(ringpop *Ringpop, nValue int, keys []string) map[string]string {
	var replicas = make(map[string]string)

	for _, value := range keys {
		for i := 0; i < nValue; i++ {
			lValue := value + "-" + strconv.Itoa(i)
			dest, ok := ringpop.Lookup(lValue)
			if ok {
				replicas[lValue] = dest
			}
		}
	}
	return replicas
}

func (r *replicator) handleOp(dest string, key string, req *forwardReq, res *forwardReqRes, doneC chan<- bool, errC chan<- error, sC chan<- int) {
	if r.ringpop.WhoAmI() == dest {
		r.ringpop.logger.WithFields(log.Fields{
			"key":  key,
			"dest": dest,
		}).Debug("[ringpop] handled locally")
		r.ringpop.emit("replicaHandledLocally")
		errC <- nil
		sC <- 0
		return
	}
	opts := map[string]interface{}{
		"dest": dest,
		"keys": []string{key},
		"req":  req,
		"res":  res,
	}

	err := r.ringpop.forwardReq(opts)
	doneC <- true
	sC <- res.StatusCode
	errC <- err
}

func (r *replicator) readWrite(rwValue int, rOpts *replicaOpts) (*replicaResp, error) {
	if rOpts == nil {
		r.ringpop.logger.WithFields(log.Fields{
			"local": r.ringpop.WhoAmI(),
		}).Debug("[ringpop] nil replica options")
		return nil, errors.New("nil options")
	}
	nValue := rOpts.nReplicas

	if nValue == 0 {
		nValue = r.nReplicas
	}

	if rwValue > nValue {
		r.ringpop.logger.WithFields(log.Fields{
			"local":   r.ringpop.WhoAmI(),
			"nValue":  nValue,
			"rwValue": rwValue,
		}).Debug("[ringpop] invalid rwValue")
		return nil, errors.New("invalid rwValue")
	}

	keys := rOpts.keys
	replicas := groupReplicas(r.ringpop, nValue, keys)

	if len(replicas) < nValue {
		r.ringpop.logger.WithFields(log.Fields{
			"local":         r.ringpop.WhoAmI(),
			"nValue":        nValue,
			"len(replicas)": len(replicas),
		}).Debug("[ringpop] not enough replicas")
		return nil, errors.New("not enough replicas")
	}

	var numErrors int
	var numResponses int
	var l sync.Mutex
	var wg sync.WaitGroup
	var err error

	resp := &replicaResp{}
	resp.destRespMap = make(map[string]int)
	for key, dest := range replicas {
		wg.Add(1)
		go func(n string, k string) {
			doneC := make(chan bool)

			var res forwardReqRes
			var lErr error
			errC := make(chan error)
			sC := make(chan int, 1)

			// attemp to handle this
			go r.handleOp(n, k, rOpts.req, &res, doneC, errC, sC)

			done := <-doneC
			if done {
				select {
				// call either succeeded or failed
				case lErr = <-errC:
					l.Lock()
					defer l.Unlock()
					resp.destRespMap[n] = <-sC
					resp.keys = append(resp.keys, k)
					if lErr != nil {
						numErrors++
					}
					numResponses++
				case <-time.After(rOpts.timeout): // timed out
					numErrors++
				}
				wg.Done()
			}

		}(dest, key)
	}
	// wait for all replicas to respond/timeout
	wg.Wait()

	if numResponses < rwValue {
		err = errors.New("replicator R/W value not satisfied")
	}

	return resp, err
}

func (r *replicator) read(rOpts *replicaOpts) (*replicaResp, error) {
	resp, err := r.readWrite(rOpts.rValue, rOpts)
	return resp, err
}

func (r *replicator) write(rOpts *replicaOpts) (*replicaResp, error) {
	resp, err := r.readWrite(rOpts.wValue, rOpts)
	return resp, err
}

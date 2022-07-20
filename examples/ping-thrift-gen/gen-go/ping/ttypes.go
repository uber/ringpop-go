// Autogenerated by Thrift Compiler (0.9.3)
// DO NOT EDIT UNLESS YOU ARE SURE THAT YOU KNOW WHAT YOU ARE DOING

package ping

import (
	"bytes"
	"fmt"
	"github.com/uber/tchannel-go/thirdparty/github.com/apache/thrift/lib/go/thrift"
)

// (needed to ensure safety because of naive import list construction.)
var _ = thrift.ZERO
var _ = fmt.Printf
var _ = bytes.Equal

var GoUnusedProtection__ int

// Attributes:
//  - Key
type Ping struct {
	Key string `thrift:"key,1,required" json:"key"`
}

func NewPing() *Ping {
	return &Ping{}
}

func (p *Ping) GetKey() string {
	return p.Key
}
func (p *Ping) Read(iprot thrift.TProtocol) error {
	if _, err := iprot.ReadStructBegin(); err != nil {
		return thrift.PrependError(fmt.Sprintf("%T read error: ", p), err)
	}

	var issetKey bool = false

	for {
		_, fieldTypeId, fieldId, err := iprot.ReadFieldBegin()
		if err != nil {
			return thrift.PrependError(fmt.Sprintf("%T field %d read error: ", p, fieldId), err)
		}
		if fieldTypeId == thrift.STOP {
			break
		}
		switch fieldId {
		case 1:
			if err := p.readField1(iprot); err != nil {
				return err
			}
			issetKey = true
		default:
			if err := iprot.Skip(fieldTypeId); err != nil {
				return err
			}
		}
		if err := iprot.ReadFieldEnd(); err != nil {
			return err
		}
	}
	if err := iprot.ReadStructEnd(); err != nil {
		return thrift.PrependError(fmt.Sprintf("%T read struct end error: ", p), err)
	}
	if !issetKey {
		return thrift.NewTProtocolExceptionWithType(thrift.INVALID_DATA, fmt.Errorf("Required field Key is not set"))
	}
	return nil
}

func (p *Ping) readField1(iprot thrift.TProtocol) error {
	if v, err := iprot.ReadString(); err != nil {
		return thrift.PrependError("error reading field 1: ", err)
	} else {
		p.Key = v
	}
	return nil
}

func (p *Ping) Write(oprot thrift.TProtocol) error {
	if err := oprot.WriteStructBegin("ping"); err != nil {
		return thrift.PrependError(fmt.Sprintf("%T write struct begin error: ", p), err)
	}
	if err := p.writeField1(oprot); err != nil {
		return err
	}
	if err := oprot.WriteFieldStop(); err != nil {
		return thrift.PrependError("write field stop error: ", err)
	}
	if err := oprot.WriteStructEnd(); err != nil {
		return thrift.PrependError("write struct stop error: ", err)
	}
	return nil
}

func (p *Ping) writeField1(oprot thrift.TProtocol) (err error) {
	if err := oprot.WriteFieldBegin("key", thrift.STRING, 1); err != nil {
		return thrift.PrependError(fmt.Sprintf("%T write field begin error 1:key: ", p), err)
	}
	if err := oprot.WriteString(string(p.Key)); err != nil {
		return thrift.PrependError(fmt.Sprintf("%T.key (1) field write error: ", p), err)
	}
	if err := oprot.WriteFieldEnd(); err != nil {
		return thrift.PrependError(fmt.Sprintf("%T write field end error 1:key: ", p), err)
	}
	return err
}

func (p *Ping) String() string {
	if p == nil {
		return "<nil>"
	}
	return fmt.Sprintf("Ping(%+v)", *p)
}

// Attributes:
//  - Message
//  - From_
//  - Pheader
type Pong struct {
	Message string  `thrift:"message,1,required" json:"message"`
	From_   string  `thrift:"from_,2,required" json:"from_"`
	Pheader *string `thrift:"pheader,3" json:"pheader,omitempty"`
}

func NewPong() *Pong {
	return &Pong{}
}

func (p *Pong) GetMessage() string {
	return p.Message
}

func (p *Pong) GetFrom_() string {
	return p.From_
}

var Pong_Pheader_DEFAULT string

func (p *Pong) GetPheader() string {
	if !p.IsSetPheader() {
		return Pong_Pheader_DEFAULT
	}
	return *p.Pheader
}
func (p *Pong) IsSetPheader() bool {
	return p.Pheader != nil
}

func (p *Pong) Read(iprot thrift.TProtocol) error {
	if _, err := iprot.ReadStructBegin(); err != nil {
		return thrift.PrependError(fmt.Sprintf("%T read error: ", p), err)
	}

	var issetMessage bool = false
	var issetFrom_ bool = false

	for {
		_, fieldTypeId, fieldId, err := iprot.ReadFieldBegin()
		if err != nil {
			return thrift.PrependError(fmt.Sprintf("%T field %d read error: ", p, fieldId), err)
		}
		if fieldTypeId == thrift.STOP {
			break
		}
		switch fieldId {
		case 1:
			if err := p.readField1(iprot); err != nil {
				return err
			}
			issetMessage = true
		case 2:
			if err := p.readField2(iprot); err != nil {
				return err
			}
			issetFrom_ = true
		case 3:
			if err := p.readField3(iprot); err != nil {
				return err
			}
		default:
			if err := iprot.Skip(fieldTypeId); err != nil {
				return err
			}
		}
		if err := iprot.ReadFieldEnd(); err != nil {
			return err
		}
	}
	if err := iprot.ReadStructEnd(); err != nil {
		return thrift.PrependError(fmt.Sprintf("%T read struct end error: ", p), err)
	}
	if !issetMessage {
		return thrift.NewTProtocolExceptionWithType(thrift.INVALID_DATA, fmt.Errorf("Required field Message is not set"))
	}
	if !issetFrom_ {
		return thrift.NewTProtocolExceptionWithType(thrift.INVALID_DATA, fmt.Errorf("Required field From_ is not set"))
	}
	return nil
}

func (p *Pong) readField1(iprot thrift.TProtocol) error {
	if v, err := iprot.ReadString(); err != nil {
		return thrift.PrependError("error reading field 1: ", err)
	} else {
		p.Message = v
	}
	return nil
}

func (p *Pong) readField2(iprot thrift.TProtocol) error {
	if v, err := iprot.ReadString(); err != nil {
		return thrift.PrependError("error reading field 2: ", err)
	} else {
		p.From_ = v
	}
	return nil
}

func (p *Pong) readField3(iprot thrift.TProtocol) error {
	if v, err := iprot.ReadString(); err != nil {
		return thrift.PrependError("error reading field 3: ", err)
	} else {
		p.Pheader = &v
	}
	return nil
}

func (p *Pong) Write(oprot thrift.TProtocol) error {
	if err := oprot.WriteStructBegin("pong"); err != nil {
		return thrift.PrependError(fmt.Sprintf("%T write struct begin error: ", p), err)
	}
	if err := p.writeField1(oprot); err != nil {
		return err
	}
	if err := p.writeField2(oprot); err != nil {
		return err
	}
	if err := p.writeField3(oprot); err != nil {
		return err
	}
	if err := oprot.WriteFieldStop(); err != nil {
		return thrift.PrependError("write field stop error: ", err)
	}
	if err := oprot.WriteStructEnd(); err != nil {
		return thrift.PrependError("write struct stop error: ", err)
	}
	return nil
}

func (p *Pong) writeField1(oprot thrift.TProtocol) (err error) {
	if err := oprot.WriteFieldBegin("message", thrift.STRING, 1); err != nil {
		return thrift.PrependError(fmt.Sprintf("%T write field begin error 1:message: ", p), err)
	}
	if err := oprot.WriteString(string(p.Message)); err != nil {
		return thrift.PrependError(fmt.Sprintf("%T.message (1) field write error: ", p), err)
	}
	if err := oprot.WriteFieldEnd(); err != nil {
		return thrift.PrependError(fmt.Sprintf("%T write field end error 1:message: ", p), err)
	}
	return err
}

func (p *Pong) writeField2(oprot thrift.TProtocol) (err error) {
	if err := oprot.WriteFieldBegin("from_", thrift.STRING, 2); err != nil {
		return thrift.PrependError(fmt.Sprintf("%T write field begin error 2:from_: ", p), err)
	}
	if err := oprot.WriteString(string(p.From_)); err != nil {
		return thrift.PrependError(fmt.Sprintf("%T.from_ (2) field write error: ", p), err)
	}
	if err := oprot.WriteFieldEnd(); err != nil {
		return thrift.PrependError(fmt.Sprintf("%T write field end error 2:from_: ", p), err)
	}
	return err
}

func (p *Pong) writeField3(oprot thrift.TProtocol) (err error) {
	if p.IsSetPheader() {
		if err := oprot.WriteFieldBegin("pheader", thrift.STRING, 3); err != nil {
			return thrift.PrependError(fmt.Sprintf("%T write field begin error 3:pheader: ", p), err)
		}
		if err := oprot.WriteString(string(*p.Pheader)); err != nil {
			return thrift.PrependError(fmt.Sprintf("%T.pheader (3) field write error: ", p), err)
		}
		if err := oprot.WriteFieldEnd(); err != nil {
			return thrift.PrependError(fmt.Sprintf("%T write field end error 3:pheader: ", p), err)
		}
	}
	return err
}

func (p *Pong) String() string {
	if p == nil {
		return "<nil>"
	}
	return fmt.Sprintf("Pong(%+v)", *p)
}

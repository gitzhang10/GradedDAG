package conn

import (
	"reflect"
	"testing"
	"time"
)

const (
	personLabel = iota
	addressLabel
)

type Person struct {
	Name string
	Age  int
}

type Address struct {
	Province string
	Town     string
	Code     int
}

var typesMap map[uint8]string

// TestSimpleComm tests if node1 (addr1, client) can connect to node2 (addr2, server) correctly
// And if node1 can send message of type 'MsgType1' to node2
// And if node2 can respond with the message of type 'MsgType1Resp' to node1
func TestSimpleComm(t *testing.T) {
	var p Person
	var addr Address
	var reflectedTypesMap = map[uint8]reflect.Type{
		personLabel:  reflect.TypeOf(p),
		addressLabel: reflect.TypeOf(addr),
	}

	person := Person{Name: "seafooler", Age: 18}

	addr1 := "127.0.0.1:8888"
	tran1, _ := NewTCPTransport(addr1, 2*time.Second, nil, 1, reflectedTypesMap)
	tran1.reflectedTypesMap = reflectedTypesMap
	defer tran1.Close()

	// Listen for a request
	go func() {
		msgWithSig := <-tran1.msgCh
		receivedPerson, ok := msgWithSig.Msg.(Person)
		if !ok {
			t.Fatal("received msg is not of type: Person")
		}
		if receivedPerson.Name != person.Name || receivedPerson.Age != person.Age {
			t.Fatal("received person does not match the original one")
		}
	}()

	addr2 := "127.0.0.1:9999"
	tran2, _ := NewTCPTransport(addr2, 2*time.Second, nil, 1, reflectedTypesMap)
	tran2.reflectedTypesMap = reflectedTypesMap
	defer tran2.Close()

	conn, err := tran2.GetConn(addr1)
	if err != nil {
		t.Errorf(err.Error())
	}

	if err := SendMsg(conn, personLabel, &person, nil); err != nil {
		t.Errorf(err.Error())
	}

	time.Sleep(time.Second)
}

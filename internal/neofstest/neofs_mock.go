package neofstest

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/nspcc-dev/neofs-s3-gw/api/layer/neofs"
	"github.com/nspcc-dev/neofs-sdk-go/container"
	cid "github.com/nspcc-dev/neofs-sdk-go/container/id"
	"github.com/nspcc-dev/neofs-sdk-go/object"
	"github.com/nspcc-dev/neofs-sdk-go/object/address"
	oid "github.com/nspcc-dev/neofs-sdk-go/object/id"
	"github.com/nspcc-dev/neofs-sdk-go/object/id/test"
	"github.com/nspcc-dev/neofs-sdk-go/owner"
)

const objectSystemAttributeName = "S3-System-name"

type TestNeoFS struct {
	neofs.NeoFS

	objects      map[string]*object.Object
	containers   map[string]*container.Container
	currentEpoch uint64
}

func NewTestNeoFS() *TestNeoFS {
	return &TestNeoFS{
		objects:    make(map[string]*object.Object),
		containers: make(map[string]*container.Container),
	}
}

func (t *TestNeoFS) CurrentEpoch() uint64 {
	return t.currentEpoch
}

func (t *TestNeoFS) Objects() []*object.Object {
	res := make([]*object.Object, 0, len(t.objects))

	for _, obj := range t.objects {
		res = append(res, obj)
	}

	return res
}

func (t *TestNeoFS) AddObject(key string, obj *object.Object) {
	t.objects[key] = obj
}

func (t *TestNeoFS) ContainerID(name string) (*cid.ID, error) {
	for id, cnr := range t.containers {
		for _, attr := range cnr.Attributes() {
			if attr.Key() == container.AttributeName && attr.Value() == name {
				cnrID := cid.New()
				return cnrID, cnrID.Parse(id)
			}
		}
	}
	return nil, fmt.Errorf("not found")
}

func (t *TestNeoFS) CreateContainer(_ context.Context, prm neofs.PrmContainerCreate) (*cid.ID, error) {
	opts := []container.Option{
		container.WithOwnerID(&prm.Creator),
		container.WithPolicy(&prm.Policy),
		container.WithCustomBasicACL(prm.BasicACL),
		container.WithAttribute(container.AttributeTimestamp, strconv.FormatInt(prm.Time.Unix(), 10)),
	}

	if prm.Name != "" {
		opts = append(opts, container.WithAttribute(container.AttributeName, prm.Name))
	}

	for _, attr := range prm.AdditionalAttributes {
		opts = append(opts, container.WithAttribute(attr[0], attr[1]))
	}

	cnr := container.New(opts...)
	cnr.SetSessionToken(prm.SessionToken)

	if prm.Name != "" {
		container.SetNativeName(cnr, prm.Name)
	}

	b := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return nil, err
	}

	id := cid.New()
	id.SetSHA256(sha256.Sum256(b))
	t.containers[id.String()] = cnr

	return id, nil
}

func (t *TestNeoFS) Container(_ context.Context, id cid.ID) (*container.Container, error) {
	for k, v := range t.containers {
		if k == id.String() {
			return v, nil
		}
	}

	return nil, fmt.Errorf("container not found " + id.String())
}

func (t *TestNeoFS) UserContainers(_ context.Context, _ owner.ID) ([]cid.ID, error) {
	var res []cid.ID
	for k := range t.containers {
		var idCnr cid.ID
		if err := idCnr.Parse(k); err != nil {
			return nil, err
		}
		res = append(res, idCnr)
	}

	return res, nil
}

func (t *TestNeoFS) SelectObjects(_ context.Context, prm neofs.PrmObjectSelect) ([]oid.ID, error) {
	filters := object.NewSearchFilters()
	filters.AddRootFilter()

	if prm.FilePrefix != "" {
		filters.AddFilter(object.AttributeFileName, prm.FilePrefix, object.MatchCommonPrefix)
	}

	if prm.ExactAttribute[0] != "" {
		filters.AddFilter(prm.ExactAttribute[0], prm.ExactAttribute[1], object.MatchStringEqual)
	}

	cidStr := prm.Container.String()

	var res []oid.ID

	if len(filters) == 1 {
		for k, v := range t.objects {
			if strings.Contains(k, cidStr) {
				res = append(res, *v.ID())
			}
		}
		return res, nil
	}

	filter := filters[1]
	if len(filters) != 2 || filter.Operation() != object.MatchStringEqual ||
		(filter.Header() != object.AttributeFileName && filter.Header() != objectSystemAttributeName) {
		return nil, fmt.Errorf("usupported filters")
	}

	for k, v := range t.objects {
		if strings.Contains(k, cidStr) && isMatched(v.Attributes(), filter) {
			res = append(res, *v.ID())
		}
	}

	return res, nil
}

func (t *TestNeoFS) ReadObject(_ context.Context, prm neofs.PrmObjectRead) (*neofs.ObjectPart, error) {
	var addr address.Address
	addr.SetContainerID(&prm.Container)
	addr.SetObjectID(&prm.Object)

	sAddr := addr.String()

	if obj, ok := t.objects[sAddr]; ok {
		return &neofs.ObjectPart{
			Head:    obj,
			Payload: io.NopCloser(bytes.NewReader(obj.Payload())),
		}, nil
	}

	return nil, fmt.Errorf("object not found " + addr.String())
}

func (t *TestNeoFS) CreateObject(_ context.Context, prm neofs.PrmObjectCreate) (*oid.ID, error) {
	id := test.ID()

	attrs := make([]*object.Attribute, 0)

	if prm.Filename != "" {
		a := object.NewAttribute()
		a.SetKey(object.AttributeFileName)
		a.SetValue(prm.Filename)
		attrs = append(attrs, a)
	}

	for i := range prm.Attributes {
		a := object.NewAttribute()
		a.SetKey(prm.Attributes[i][0])
		a.SetValue(prm.Attributes[i][1])
		attrs = append(attrs, a)
	}

	obj := object.New()
	obj.SetContainerID(&prm.Container)
	obj.SetID(id)
	obj.SetPayloadSize(prm.PayloadSize)
	obj.SetAttributes(attrs...)
	obj.SetCreationEpoch(t.currentEpoch)
	t.currentEpoch++

	if prm.Payload != nil {
		all, err := io.ReadAll(prm.Payload)
		if err != nil {
			return nil, err
		}
		obj.SetPayload(all)
		obj.SetPayloadSize(uint64(len(all)))
	}

	addr := newAddress(obj.ContainerID(), obj.ID())
	t.objects[addr.String()] = obj
	return obj.ID(), nil
}

func (t *TestNeoFS) DeleteObject(_ context.Context, prm neofs.PrmObjectDelete) error {
	var addr address.Address
	addr.SetContainerID(&prm.Container)
	addr.SetObjectID(&prm.Object)

	delete(t.objects, addr.String())

	return nil
}

func isMatched(attributes []*object.Attribute, filter object.SearchFilter) bool {
	for _, attr := range attributes {
		if attr.Key() == filter.Header() && attr.Value() == filter.Value() {
			return true
		}
	}

	return false
}

func newAddress(cid *cid.ID, oid *oid.ID) *address.Address {
	addr := address.NewAddress()
	addr.SetContainerID(cid)
	addr.SetObjectID(oid)
	return addr
}
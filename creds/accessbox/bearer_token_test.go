package accessbox

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"testing"

	"github.com/nspcc-dev/neofs-api-go/pkg/acl/eacl"
	"github.com/nspcc-dev/neofs-api-go/pkg/token"
	"github.com/nspcc-dev/neofs-s3-gw/creds/hcs"
	"github.com/stretchr/testify/require"
)

func Test_tokens_encode_decode(t *testing.T) {
	var (
		tkn  = token.NewBearerToken()
		tkn2 = token.NewBearerToken()
	)
	sec, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	cred, err := hcs.Generate(rand.Reader)
	require.NoError(t, err)

	tkn.SetEACLTable(eacl.NewTable())
	require.NoError(t, tkn.SignToken(sec))

	data, err := encodeToken(tkn, cred.PrivateKey(), cred.PublicKey())
	require.NoError(t, err)

	err = decodeToken(data, tkn2, cred.PrivateKey(), cred.PublicKey())
	require.NoError(t, err)

	require.Equal(t, tkn, tkn2)
}

func Test_bearer_token_in_access_box(t *testing.T) {
	var (
		box, box2 AccessBox
		tkn       = token.NewBearerToken()
	)

	sec, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	cred, err := hcs.Generate(rand.Reader)
	require.NoError(t, err)

	tkn.SetEACLTable(eacl.NewTable())
	require.NoError(t, tkn.SignToken(sec))

	box.SetOwnerPublicKey(cred.PublicKey())

	err = box.AddBearerToken(tkn, cred.PrivateKey(), cred.PublicKey())
	require.NoError(t, err)

	data, err := box.Marshal()
	require.NoError(t, err)

	err = box2.Unmarshal(data)
	require.NoError(t, err)

	tkn2, err := box2.GetBearerToken(cred.PrivateKey())
	require.NoError(t, err)

	require.Equal(t, tkn, tkn2)
}

func Test_accessbox_multiple_keys(t *testing.T) {
	var (
		box AccessBox
		tkn = token.NewBearerToken()
	)

	sec, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	cred, err := hcs.Generate(rand.Reader)
	require.NoError(t, err)

	tkn.SetEACLTable(eacl.NewTable())
	require.NoError(t, tkn.SignToken(sec))

	count := 10
	pubs := make([]hcs.PublicKey, 0, count)
	keys := make([]hcs.PrivateKey, 0, count)
	{ // generate keys
		for i := 0; i < count; i++ {
			cred, err := hcs.Generate(rand.Reader)
			require.NoError(t, err)

			pubs = append(pubs, cred.PublicKey())
			keys = append(keys, cred.PrivateKey())
		}
	}

	box.SetOwnerPublicKey(cred.PublicKey())

	err = box.AddBearerToken(tkn, cred.PrivateKey(), pubs...)
	require.NoError(t, err)

	for i, k := range keys {
		tkn2, err := box.GetBearerToken(k)
		require.NoError(t, err, "key #%d: %s failed", i, k)
		require.Equal(t, tkn2, tkn)
	}
}

func Test_unknown_key(t *testing.T) {
	var (
		box AccessBox
		tkn = token.NewBearerToken()
	)

	sec, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	cred, err := hcs.Generate(rand.Reader)
	require.NoError(t, err)

	wrongCred, err := hcs.Generate(rand.Reader)
	require.NoError(t, err)

	tkn.SetEACLTable(eacl.NewTable())
	require.NoError(t, tkn.SignToken(sec))

	box.SetOwnerPublicKey(cred.PublicKey())

	err = box.AddBearerToken(tkn, cred.PrivateKey(), cred.PublicKey())
	require.NoError(t, err)

	_, err = box.GetBearerToken(wrongCred.PrivateKey())
	require.Error(t, err)
}
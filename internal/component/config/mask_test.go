package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const validBcryptHash = "$2a$10$abcdefghijklmnopqrstuuABCDEFGHIJKLMNOPQRSTUVWXYZ01234"

// maskTestSchema has, under system.authentication.user{name}, a ze:bcrypt
// "password", a ze:sensitive "api-secret", and a plain "shell" leaf.
func maskTestSchema() *Schema {
	schema := NewSchema()

	pw := Leaf(TypeString)
	pw.Bcrypt = true
	secret := Leaf(TypeString)
	secret.Sensitive = true
	plain := Leaf(TypeString)

	users := List(TypeString,
		Field("password", pw),
		Field("api-secret", secret),
		Field("shell", plain),
	)
	users.KeyName = "name"

	auth := Container(Field("user", users))
	sys := Container(Field("authentication", auth))
	schema.Define("system", sys)
	return schema
}

func maskTreeWithUser(t *testing.T, password string) *Tree {
	t.Helper()
	tree := NewTree()
	auth := tree.GetOrCreateContainer("system").GetOrCreateContainer("authentication")
	entry := NewTree()
	if password != "" {
		entry.Set("password", password)
	}
	entry.Set("api-secret", "topsecret")
	entry.Set("shell", "/bin/sh")
	auth.AddListEntry("user", "alice", entry)
	return tree
}

func aliceOf(tree *Tree) *Tree {
	return tree.GetContainer("system").GetContainer("authentication").GetList("user")["alice"]
}

// VALIDATES: AC-5 — MaskBcrypt replaces bcrypt leaf values with the placeholder,
// leaves sensitive/plain leaves untouched, and does not mutate the input tree.
func TestMaskBcryptLeavesForDisplay(t *testing.T) {
	schema := maskTestSchema()
	tree := maskTreeWithUser(t, validBcryptHash)

	masked := MaskBcrypt(tree, schema)

	// Masked clone: bcrypt leaf hidden.
	mAlice := aliceOf(masked)
	require.NotNil(t, mAlice)
	got, _ := mAlice.Get("password")
	assert.Equal(t, SecretDataPlaceholder, got, "bcrypt leaf must be masked")
	// Non-bcrypt leaves untouched by MaskBcrypt.
	secret, _ := mAlice.Get("api-secret")
	assert.Equal(t, "topsecret", secret, "sensitive leaf must not be touched by the bcrypt mask")
	shell, _ := mAlice.Get("shell")
	assert.Equal(t, "/bin/sh", shell, "plain leaf must not be touched")

	// Original tree unmodified (clone semantics).
	oAlice := aliceOf(tree)
	orig, _ := oAlice.Get("password")
	assert.Equal(t, validBcryptHash, orig, "MaskBcrypt must not mutate the input tree")
}

// VALIDATES: an empty bcrypt leaf is not turned into the placeholder.
func TestMaskBcryptSkipsEmpty(t *testing.T) {
	schema := maskTestSchema()
	tree := maskTreeWithUser(t, "") // no password set

	masked := MaskBcrypt(tree, schema)
	_, present := aliceOf(masked).Get("password")
	assert.False(t, present, "empty/absent bcrypt leaf must stay absent, not become the placeholder")
}

// VALIDATES: nil-safety.
func TestMaskBcryptNilInputs(t *testing.T) {
	assert.Nil(t, MaskBcrypt(nil, nil))
	tree := NewTree()
	assert.Same(t, tree, MaskBcrypt(tree, nil), "nil schema returns the tree unchanged")
}

// VALIDATES: MaskSecretsInPlace masks every secret leaf of a tree the caller
// already owns, and answers on the same predicate as MaskSecrets.
// PREVENTS: the in-place mask drifting narrower than the cloning one. The two
// used to answer different questions, so `show | display active` published a
// ze:sensitive value that plain `show` masked.
func TestMaskSecretsInPlaceMasksTheSameLeaves(t *testing.T) {
	schema := maskTestSchema()
	tree := maskTreeWithUser(t, validBcryptHash)

	MaskSecretsInPlace(tree, schema)

	alice := aliceOf(tree)
	require.NotNil(t, alice)
	password, _ := alice.Get("password")
	assert.Equal(t, SecretDataPlaceholder, password, "the bcrypt leaf must be masked")
	secret, _ := alice.Get("api-secret")
	assert.Equal(t, SecretDataPlaceholder, secret, "the sensitive leaf must be masked")
	shell, _ := alice.Get("shell")
	assert.Equal(t, "/bin/sh", shell, "a plain leaf must not be touched")

	empty := maskTreeWithUser(t, "")
	MaskSecretsInPlace(empty, schema)
	_, present := aliceOf(empty).Get("password")
	assert.False(t, present, "an absent secret leaf must stay absent, not become the placeholder")

	MaskSecretsInPlace(nil, schema)
	MaskSecretsInPlace(NewTree(), nil)
}

// VALIDATES: AC-8 — the placeholder guard rejects a bcrypt leaf whose value is
// exactly the display placeholder, naming the offending leaf.
func TestRejectMaskedSecretLeaves(t *testing.T) {
	schema := maskTestSchema()

	// A tree carrying the placeholder on the bcrypt leaf is rejected.
	bad := maskTreeWithUser(t, SecretDataPlaceholder)
	err := RejectMaskedSecretLeaves(bad, schema)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "alice")
	assert.Contains(t, err.Error(), "password")
	assert.Contains(t, err.Error(), "plaintext-")

	// A tree with the real hash passes.
	good := maskTreeWithUser(t, validBcryptHash)
	assert.NoError(t, RejectMaskedSecretLeaves(good, schema))

	// Empty / nil are no-ops.
	assert.NoError(t, RejectMaskedSecretLeaves(NewTree(), schema))
	assert.NoError(t, RejectMaskedSecretLeaves(nil, nil))
}

// VALIDATES: MaskSecrets replaces every secret leaf value with the placeholder,
// leaves a plain leaf alone, and does not mutate the input tree.
// PREVENTS: a display path that renders a ze:sensitive value. The only whole-tree
// mask was MaskBcrypt, so the web CLI terminal serialized every ze:sensitive leaf
// in the clear to any authenticated session.
func TestMaskSecretsCoversEverySecretLeaf(t *testing.T) {
	schema := maskTestSchema()
	tree := maskTreeWithUser(t, validBcryptHash)

	masked := aliceOf(MaskSecrets(tree, schema))
	require.NotNil(t, masked)

	hash, _ := masked.Get("password")
	assert.Equal(t, SecretDataPlaceholder, hash, "a ze:bcrypt leaf must be masked")
	secret, _ := masked.Get("api-secret")
	assert.Equal(t, SecretDataPlaceholder, secret, "a ze:sensitive leaf must be masked")
	shell, _ := masked.Get("shell")
	assert.Equal(t, "/bin/sh", shell, "a plain leaf must be rendered")

	original, _ := aliceOf(tree).Get("api-secret")
	assert.Equal(t, "topsecret", original, "MaskSecrets must not mutate the input tree")

	assert.Nil(t, MaskSecrets(nil, nil))
	empty := NewTree()
	assert.Same(t, empty, MaskSecrets(empty, nil), "a nil schema returns the tree unchanged")
}

// VALIDATES: the write guard refuses the placeholder on every leaf the display
// mask can write it to.
// PREVENTS: a dump-then-load round trip storing the placeholder as the secret.
// `ze config dump --strip` writes it for a ze:sensitive leaf, and the guard read
// ze:bcrypt alone, so loading that dump replaced the secret with 17 bytes of C
// comment. The read half and the write half answer one predicate now.
func TestRejectMaskedSecretLeavesCoversTheSameLeavesAsTheMask(t *testing.T) {
	schema := maskTestSchema()

	tree := maskTreeWithUser(t, validBcryptHash)
	aliceOf(tree).Set("api-secret", SecretDataPlaceholder)

	err := RejectMaskedSecretLeaves(tree, schema)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "api-secret", "a masked ze:sensitive leaf must be named")

	aliceOf(tree).Set("api-secret", "topsecret")
	assert.NoError(t, RejectMaskedSecretLeaves(tree, schema))
}

// VALIDATES: ChangedSecretPaths names a secret leaf whose value moved between
// two trees, and names nothing else.
// PREVENTS: a masked compare stating something false. Both sides render the same
// placeholder, so a diff of the two masked texts is empty and the verb answered
// "(no changes)" over a rotated password. The operator must learn that the leaf
// moved, and neither value.
func TestChangedSecretPathsNamesARotatedSecret(t *testing.T) {
	schema := maskTestSchema()
	base := maskTreeWithUser(t, validBcryptHash)
	work := maskTreeWithUser(t, validBcryptHash)
	aliceOf(work).Set("api-secret", "rotated-secret")

	assert.Equal(t, []string{"system.authentication.user.alice.api-secret"},
		ChangedSecretPaths(base, work, schema))

	// The two masked texts are equal, which is the hole this closes.
	assert.Equal(t,
		Serialize(MaskSecrets(base, schema), schema),
		Serialize(MaskSecrets(work, schema), schema),
		"the mask must hide the change, or this test proves nothing")
}

// VALIDATES: ChangedSecretPaths stays silent on what the text diff already shows.
// PREVENTS: a second report of the same change. An added or removed secret line
// is visible in the masked text, because an empty value renders no placeholder.
func TestChangedSecretPathsReportsOnlyWhatTheMaskHides(t *testing.T) {
	schema := maskTestSchema()
	base := maskTreeWithUser(t, validBcryptHash)

	assert.Empty(t, ChangedSecretPaths(base, maskTreeWithUser(t, validBcryptHash), schema),
		"two equal trees hold no changed secret")

	plain := maskTreeWithUser(t, validBcryptHash)
	aliceOf(plain).Set("shell", "/bin/zsh")
	assert.Empty(t, ChangedSecretPaths(base, plain, schema),
		"a plain leaf is the text diff's business")

	unset := maskTreeWithUser(t, "")
	assert.Empty(t, ChangedSecretPaths(unset, base, schema),
		"a secret set on one side alone already reads as an added line")

	assert.Nil(t, ChangedSecretPaths(base, nil, schema))
	assert.Nil(t, ChangedSecretPaths(base, base, nil))
}

// VALIDATES: ChangedSecretPathsSubtree answers paths relative to its node.
// PREVENTS: a compare at a context path reporting a leaf by its root path, or
// reporting a change from outside the section the operator asked about.
func TestChangedSecretPathsSubtreeIsRelativeToItsNode(t *testing.T) {
	schema := maskTestSchema()
	base := maskTreeWithUser(t, validBcryptHash)
	work := maskTreeWithUser(t, validBcryptHash)
	aliceOf(work).Set("password", "$2a$10$0123456789012345678901uvwxyzABCDEFGHIJKLMNOPQRSTUV")

	node := schema.Get("system")
	require.NotNil(t, node)

	assert.Equal(t, []string{"authentication.user.alice.password"},
		ChangedSecretPathsSubtree(base.GetContainer("system"), work.GetContainer("system"), node))
	assert.Nil(t, ChangedSecretPathsSubtree(base, work, nil))
}

// VALIDATES: LeafHoldsSecret reads ze:sensitive and ze:bcrypt, and nothing else.
// PREVENTS: the predicate drifting to ze:ephemeral, which answers whether a value
// is persisted. A non-secret ephemeral leaf would then render the placeholder,
// and its paired write guard would drop what the operator typed there.
func TestLeafHoldsSecretReadsTheTwoSecretExtensions(t *testing.T) {
	sensitive := Leaf(TypeString)
	sensitive.Sensitive = true
	bcrypt := Leaf(TypeString)
	bcrypt.Bcrypt = true
	ephemeral := Leaf(TypeString)
	ephemeral.Ephemeral = true

	assert.True(t, LeafHoldsSecret(sensitive))
	assert.True(t, LeafHoldsSecret(bcrypt))
	assert.False(t, LeafHoldsSecret(ephemeral), "ze:ephemeral says nothing about secrecy")
	assert.False(t, LeafHoldsSecret(Leaf(TypeString)))
	assert.False(t, LeafHoldsSecret(nil), "an unresolved node holds no secret")
}

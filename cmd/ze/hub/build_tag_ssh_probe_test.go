//go:build !ze_ssh

package hub

// These fixtures intentionally live in an untagged test file. Both sides of
// the ze_ssh boundary therefore exercise identical operator syntax without
// importing the gated SSH package.
const sshSharedIdentityConfig = `
system {
	authentication {
		user operator {
			password "$2a$10$abcdefghijklmnopqrstuuABCDEFGHIJKLMNOPQRSTUVWXYZ012"
			profile [ readonly ]
		}
	}
	authorization {
		profile readonly {
			run { default-action allow; }
			edit { default-action deny; }
		}
	}
}
`

const sshOnlyTransportConfig = `
environment {
	ssh {
		enabled true
		server main {
			ip 127.0.0.1
			port 2222
		}
	}
}
`

const sshPublicKeyIdentityConfig = `
system {
	authentication {
		user operator {
			password "$2a$10$abcdefghijklmnopqrstuuABCDEFGHIJKLMNOPQRSTUVWXYZ012"
			profile [ readonly ]
			public-keys laptop {
				type ssh-ed25519
				key AAAAC3NzaC1lZDI1NTE5AAAAIExampleKeyDataHere
			}
		}
	}
	authorization {
		profile readonly {
			run { default-action allow; }
			edit { default-action deny; }
		}
	}
}
`

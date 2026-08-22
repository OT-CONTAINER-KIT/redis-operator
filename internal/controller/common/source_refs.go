package common

import (
	commonapi "github.com/OT-CONTAINER-KIT/redis-operator/api/common/v1beta2"
)

// ReferencedSecretNames returns the names of the user-provided Secrets the operator
// mounts or references for a Redis workload: the TLS certificate Secret, the ACL
// Secret, and the password Secret (ExistingPasswordSecret). References that are not
// set (or set with an empty name) are skipped, so the result only contains real
// Secret names a controller should watch.
func ReferencedSecretNames(tls *commonapi.TLSConfig, acl *commonapi.ACLConfig, password *commonapi.ExistingPasswordSecret) []string {
	var names []string
	if tls != nil && tls.Secret.SecretName != "" {
		names = append(names, tls.Secret.SecretName)
	}
	if acl != nil && acl.Secret != nil && acl.Secret.SecretName != "" {
		names = append(names, acl.Secret.SecretName)
	}
	if password != nil && password.Name != nil && *password.Name != "" {
		names = append(names, *password.Name)
	}
	return names
}

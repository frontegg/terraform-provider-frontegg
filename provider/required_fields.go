package provider

type fronteggFieldGetter interface {
	Get(key string) interface{}
}

type fronteggKnownValueChecker interface {
	NewValueKnown(key string) bool
}

func valueUnknown(d fronteggFieldGetter, key string) bool {
	checker, ok := d.(fronteggKnownValueChecker)
	return ok && !checker.NewValueKnown(key)
}

func stringValueMissing(d fronteggFieldGetter, key string) bool {
	return !valueUnknown(d, key) && d.Get(key).(string) == ""
}

func secretValueMissing(d fronteggFieldGetter, name string) bool {
	if valueUnknown(d, name+writeOnlyVersionSuffix) {
		return false
	}
	return stringValueMissing(d, name) && !usesWriteOnlySecret(d, name)
}

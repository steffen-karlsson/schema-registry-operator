package v1alpha1

type Updatable interface {
	UpdateStatus(ready bool, message string)
}

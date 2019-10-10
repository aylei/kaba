package storage

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"k8s.io/api/core/v1"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/conversion"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/apiserver/pkg/storage"
	"k8s.io/apiserver/pkg/storage/etcd"
	// informerv1 "k8s.io/client-go/informers/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

const (
	labelDomain = "aba.pingcap.com"
)

var (
	groupLabel = labelDomain + "/group"
	kindLabel = labelDomain + "/kind"
	namespaceLabel = labelDomain + "/namespace"
	nameLabel      = labelDomain + "/name"
)

// objKey is a structured representation of key '/{group}/{kind}/{ns}/{name}'
type objKey struct {
	group     string
	kind      string
	namespace string
	name      string
	fullName  string
}

func (o *objKey) objectMeta() metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name: o.fullName,
		Labels: map[string]string{
			groupLabel:     o.group,
			kindLabel:      o.kind,
			namespaceLabel: o.namespace,
			nameLabel:      o.name,
		},
	}
}

// store implements a ConfigMap backed storage.Interface
type store struct {
	// informer informerv1.ConfigMapInformer
	kubeCli   corev1.ConfigMapInterface
	codec     runtime.Codec
	versioner storage.Versioner
}

// New returns an kubernetes configmap implementation of storage.Interface.
func New(kubeCli kubernetes.Interface, codec runtime.Codec, namespace string) storage.Interface {
	return &store{
		versioner: etcd.APIObjectVersioner{},
		kubeCli:   kubeCli.CoreV1().ConfigMaps(namespace),
		codec:     codec,
	}
}

func (s *store) Versioner() storage.Versioner {
	return s.versioner
}

// Create adds a new object at a key unless it already exists. 'ttl' is time-to-live
// in seconds (0 means forever). If no error is returned and out is not nil, out will be
// set to the read value from database.
func (s *store) Create(ctx context.Context, key string, obj, out runtime.Object, ttl uint64) error {
	if version, err := s.versioner.ObjectResourceVersion(obj); err == nil && version != 0 {
		return errors.New("resourceVersion should not be set on objects to be created")
	}
	if err := s.versioner.PrepareObjectForStorage(obj); err != nil {
		return fmt.Errorf("PrepareObjectForStorage failed: %v", err)
	}
	data, err := runtime.Encode(s.codec, obj)
	if err != nil {
		return err
	}
	configData := map[string]string{
		"value": string(data),
	}
	objKey := parseObjKey(key)
	cm, err := s.kubeCli.Create(&v1.ConfigMap{
		ObjectMeta: objKey.objectMeta(),
		Data: configData,
	})
	if err != nil {
		return err
	}
	if out != nil {
		rv, err := s.versioner.ParseResourceVersion(cm.ResourceVersion)
		if err != nil {
			return err
		}
		err = decode(s.codec, s.versioner, []byte(cm.Data["value"]), out, int64(rv))
		return err
	}
	return nil
}

// Delete removes the specified key and returns the value that existed at that spot.
// If key didn't exist, it will return NotFound storage error.
func (s *store) Delete(ctx context.Context, key string, out runtime.Object, preconditions *storage.Preconditions) error {
	if err := s.kubeCli.Delete(key, &metav1.DeleteOptions{
		Preconditions: &metav1.Preconditions{
			UID: preconditions.UID,
		},
	}); err != nil {
		return err
	}
	return nil
}

// Watch begins watching the specified key. Events are decoded into API objects,
// and any items selected by 'p' are sent down to returned watch.Interface.
// resourceVersion may be used to specify what version to begin watching,
// which should be the current resourceVersion, and no longer rv+1
// (e.g. reconnecting without missing any updates).
// If resource version is "0", this interface will get current object at given key
// and send it in an "ADDED" event, before watch starts.
func (s *store) Watch(ctx context.Context, key string, resourceVersion string, p storage.SelectionPredicate) (watch.Interface, error) {
	return nil, nil
}

// WatchList begins watching the specified key's items. Items are decoded into API
// objects and any item selected by 'p' are sent down to returned watch.Interface.
// resourceVersion may be used to specify what version to begin watching,
// which should be the current resourceVersion, and no longer rv+1
// (e.g. reconnecting without missing any updates).
// If resource version is "0", this interface will list current objects directory defined by key
// and send them in "ADDED" events, before watch starts.
func (s *store) WatchList(ctx context.Context, key string, resourceVersion string, p storage.SelectionPredicate) (watch.Interface, error) {
	return nil, nil
}

// Get unmarshals json found at key into objPtr. On a not found error, will either
// return a zero object of the requested type, or an error, depending on ignoreNotFound.
// Treats empty responses and nil response nodes exactly like a not found error.
// The returned contents may be delayed, but it is guaranteed that they will
// be have at least 'resourceVersion'.
func (s *store) Get(ctx context.Context, key string, resourceVersion string, out runtime.Object, ignoreNotFound bool) error {
	objKey := parseObjKey(key)
	cm, err := s.kubeCli.Get(objKey.fullName, metav1.GetOptions{})
	if err != nil {
		if apierr.IsNotFound(err) {
			if ignoreNotFound {
				return runtime.SetZeroValue(out)
			} else {
				return storage.NewKeyNotFoundError(key, 0)
			}
		} else {
			return err
		}
	}
	rv, err := s.versioner.ParseResourceVersion(cm.ResourceVersion)
	if err != nil {
		return err
	}
	return decode(s.codec, s.versioner, []byte(cm.Data["value"]), out, int64(rv))
}

// GetToList unmarshals json found at key and opaque it into *List api object
// (an object that satisfies the runtime.IsList definition).
// The returned contents may be delayed, but it is guaranteed that they will
// be have at least 'resourceVersion'.
func (s *store) GetToList(ctx context.Context, key string, resourceVersion string, p storage.SelectionPredicate, listObj runtime.Object) error {
	listPtr, err := meta.GetItemsPtr(listObj)
	if err != nil {
		return err
	}
	v, err := conversion.EnforcePtr(listPtr)
	if err != nil || v.Kind() != reflect.Slice {
		panic("need ptr to slice")
	}
	objKey := parseObjKey(key)
	cmList, err := s.kubeCli.List(metav1.ListOptions{
		FieldSelector: fmt.Sprintf("metada.name=%s", objKey.fullName),
	})
	if err != nil {
		return err
	}
	if len(cmList.Items) > 0 {
		rv, err := s.versioner.ParseResourceVersion(cmList.Items[0].ResourceVersion)
		if err != nil {
			return err
		}
		if err := appendListItem(v, []byte(cmList.Items[0].Data["value"]), rv, p, s.codec, s.versioner); err != nil {
			return err
		}
	}
	listRV, err := s.versioner.ParseResourceVersion(cmList.ResourceVersion)
	if err != nil {
		return nil
	}

	return s.versioner.UpdateList(listObj, listRV, "")
}

// List unmarshalls jsons found at directory defined by key and opaque them
// into *List api object (an object that satisfies runtime.IsList definition).
// The returned contents may be delayed, but it is guaranteed that they will
// be have at least 'resourceVersion'.
func (s *store) List(ctx context.Context, key string, resourceVersion string, p storage.SelectionPredicate, listObj runtime.Object) error {
	listPtr, err := meta.GetItemsPtr(listObj)
	if err != nil {
		return err
	}
	v, err := conversion.EnforcePtr(listPtr)
	if err != nil || v.Kind() != reflect.Slice {
		panic("need ptr to slice")
	}
	// TODO implemention
	return nil
}

// GuaranteedUpdate keeps calling 'tryUpdate()' to update key 'key' (of type 'ptrToType')
// retrying the update until success if there is index conflict.
// Note that object passed to tryUpdate may change across invocations of tryUpdate() if
// other writers are simultaneously updating it, so tryUpdate() needs to take into account
// the current contents of the object when deciding how the update object should look.
// If the key doesn't exist, it will return NotFound storage error if ignoreNotFound=false
// or zero value in 'ptrToType' parameter otherwise.
// If the object to update has the same value as previous, it won't do any update
// but will return the object in 'ptrToType' parameter.
// If 'suggestion' can contain zero or one element - in such case this can be used as
// a suggestion about the current version of the object to avoid read operation from
// storage to get it.
//
// Example:
//
// s := /* implementation of Interface */
// err := s.GuaranteedUpdate(
//     "myKey", &MyType{}, true,
//     func(input runtime.Object, res ResponseMeta) (runtime.Object, *uint64, error) {
//       // Before each incovation of the user defined function, "input" is reset to
//       // current contents for "myKey" in database.
//       curr := input.(*MyType)  // Guaranteed to succeed.
//
//       // Make the modification
//       curr.Counter++
//
//       // Return the modified object - return an error to stop iterating. Return
//       // a uint64 to alter the TTL on the object, or nil to keep it the same value.
//       return cur, nil, nil
//    }
// })
func (s *store) GuaranteedUpdate(ctx context.Context,
	key string,
	ptrToType runtime.Object,
	ignoreNotFound bool,
	precondtions *storage.Preconditions,
	tryUpdate storage.UpdateFunc,
	suggestion ...runtime.Object,
) error {

	// TODO implementation
	return nil
}

// Count returns number of different entries under the key (generally being path prefix).
func (s *store) Count(key string) (int64, error) {

	return 0, nil
}

// decode decodes value of bytes into object. It will also set the object resource version to rev.
// On success, objPtr would be set to the object.
func decode(codec runtime.Codec, versioner storage.Versioner, value []byte, objPtr runtime.Object, rev int64) error {
	if _, err := conversion.EnforcePtr(objPtr); err != nil {
		panic("unable to convert output object to pointer")
	}
	_, _, err := codec.Decode(value, nil, objPtr)
	if err != nil {
		return err
	}
	// being unable to set the version does not prevent the object from being extracted
	versioner.UpdateObject(objPtr, uint64(rev))
	return nil
}

// appendListItem decodes and appends the object (if it passes filter) to v, which must be a slice.
func appendListItem(v reflect.Value, data []byte, rev uint64, pred storage.SelectionPredicate, codec runtime.Codec, versioner storage.Versioner) error {
	obj, _, err := codec.Decode(data, nil, reflect.New(v.Type().Elem()).Interface().(runtime.Object))
	if err != nil {
		return err
	}
	// being unable to set the version does not prevent the object from being extracted
	versioner.UpdateObject(obj, rev)
	if matched, err := pred.Matches(obj); err == nil && matched {
		v.Set(reflect.Append(v, reflect.ValueOf(obj).Elem()))
	}
	return nil
}

// parseObjKey parse a string key with the '/{group}/{kind}/{ns}/{name}' format to an objKey struct
func parseObjKey(key string) *objKey {
	s := strings.Split(strings.TrimPrefix(key, "/"), "/")
	if len(s) != 4 {
		panic("expect a string key in /{group}/{kind}/{ns}/{name} format")
	}
	return &objKey{
		group:     s[0],
		kind:      s[1],
		namespace: s[2],
		name:      s[3],
		fullName:  strings.ToLower(strings.Join(s, "-")),
	}
}

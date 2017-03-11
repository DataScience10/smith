package app

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/atlassian/smith"
	"github.com/atlassian/smith/pkg/resources"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	apiv1 "k8s.io/client-go/pkg/api/v1"
	extensions "k8s.io/client-go/pkg/apis/extensions/v1beta1"
	"k8s.io/client-go/tools/cache"
)

type watchState struct {
	cancel context.CancelFunc
}

// tprEventHandler handles events for objects with Kind: ThirdPartyResource.
// For each object a new informer is started to watch for events.
type tprEventHandler struct {
	ctx      context.Context
	clients  dynamic.ClientPool
	handler  cache.ResourceEventHandler
	mx       sync.Mutex
	watchers map[string]watchState
}

func newTprEventHandler(ctx context.Context, handler cache.ResourceEventHandler, clients dynamic.ClientPool) *tprEventHandler {
	return &tprEventHandler{
		ctx:      ctx,
		clients:  clients,
		handler:  handler,
		watchers: make(map[string]watchState),
	}
}

func (h *tprEventHandler) OnAdd(obj interface{}) {
	h.mx.Lock()
	defer h.mx.Unlock()
	h.onAdd(obj)
	// TODO rebuild all bundles containing resources of this type
}

func (h *tprEventHandler) OnUpdate(oldObj, newObj interface{}) {
	h.mx.Lock()
	defer h.mx.Unlock()
	h.onDelete(oldObj)
	h.onAdd(newObj)
	// TODO rebuild all bundles containing resources of this type
}

func (h *tprEventHandler) OnDelete(obj interface{}) {
	h.mx.Lock()
	defer h.mx.Unlock()
	h.onDelete(obj)
	// TODO rebuild all bundles containing resources of this type
}

func (h *tprEventHandler) onAdd(obj interface{}) {
	tpr := obj.(*extensions.ThirdPartyResource)
	if tpr.Name == smith.BundleResourceName {
		log.Printf("[TPREH] Not watching known TPR %s", tpr.Name)
		return
	}
	log.Printf("[TPREH] Handling OnAdd for TPR %s", tpr.Name)
	path, groupKind := resources.SplitTprName(tpr.Name)
	for _, version := range tpr.Versions {
		dc, err := h.clients.ClientForGroupVersionKind(schema.GroupVersionKind{
			Group:   groupKind.Group,
			Version: version.Name,
			Kind:    groupKind.Kind,
		})
		if err != nil {
			log.Printf("[TPREH] Failed to instantiate client for TPR %s of version %s: %v", tpr.Name, version.Name, err)
			continue
		}
		res := dc.Resource(&metav1.APIResource{
			Name: path,
			Kind: groupKind.Kind,
		}, apiv1.NamespaceAll)
		tprInf := cache.NewSharedInformer(&cache.ListWatch{
			ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
				return res.List(options)
			},
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				return res.Watch(options)
			},
		}, &unstructured.Unstructured{}, 0)

		tprInf.AddEventHandler(h.handler)

		ctx, cancel := context.WithCancel(h.ctx)
		h.watchers[key(tpr.Name, version.Name)] = watchState{cancel}

		go tprInf.Run(ctx.Done())
	}
}

func (h *tprEventHandler) onDelete(obj interface{}) {
	tpr := obj.(*extensions.ThirdPartyResource)
	for _, version := range tpr.Versions {
		k := key(tpr.Name, version.Name)
		ws, ok := h.watchers[k]
		if ok {
			delete(h.watchers, k)
			ws.cancel()
		}
	}
}

func key(name, version string) string {
	return fmt.Sprintf("%s|%s", name, version)
}
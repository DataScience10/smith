package types

import (
	"github.com/atlassian/smith/pkg/cleanup"

	sc_v1b1 "github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog/v1beta1"
	apps_v1b1 "k8s.io/api/apps/v1beta1"
	core_v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	unstructured_conversion "k8s.io/apimachinery/pkg/conversion/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var MainKnownTypes = map[schema.GroupKind]cleanup.SpecCleanup{
	{Group: apps_v1b1.GroupName, Kind: "Deployment"}: deploymentCleanup,
	{Group: core_v1.GroupName, Kind: "Service"}:      serviceCleanup,
}

var ServiceCatalogKnownTypes = map[schema.GroupKind]cleanup.SpecCleanup{
	{Group: sc_v1b1.GroupName, Kind: "ServiceBinding"}:  scServiceBindingCleanup,
	{Group: sc_v1b1.GroupName, Kind: "ServiceInstance"}: scServiceInstanceCleanup,
}

func deploymentCleanup(spec, actual *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	var deployment apps_v1b1.Deployment
	if err := unstructured_conversion.DefaultConverter.FromUnstructured(spec.Object, &deployment); err != nil {
		return nil, err
	}

	// TODO: implement cleanup

	return spec, nil
}

func serviceCleanup(spec, actual *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	var serviceSpec core_v1.Service
	if err := unstructured_conversion.DefaultConverter.FromUnstructured(spec.Object, &serviceSpec); err != nil {
		return nil, err
	}
	var serviceActual core_v1.Service
	if err := unstructured_conversion.DefaultConverter.FromUnstructured(actual.Object, &serviceActual); err != nil {
		return nil, err
	}

	serviceSpec.Spec.ClusterIP = serviceActual.Spec.ClusterIP
	serviceSpec.Status = serviceActual.Status

	if len(serviceActual.Spec.Ports) == len(serviceSpec.Spec.Ports) {
		for i, port := range serviceSpec.Spec.Ports {
			if port.NodePort == 0 {
				actualPort := serviceActual.Spec.Ports[i]
				port.NodePort = actualPort.NodePort
				if port == actualPort { // NodePort field is the only difference, other fields are the same
					// Copy port from actual if port is not specified in spec
					serviceSpec.Spec.Ports[i].NodePort = actualPort.NodePort
				}
			}
		}
	}

	u, err := unstructured_conversion.DefaultConverter.ToUnstructured(&serviceSpec)
	if err != nil {
		return nil, err
	}
	return &unstructured.Unstructured{
		Object: u,
	}, nil
}

func scServiceBindingCleanup(spec, actual *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	var sicSpec sc_v1b1.ServiceBinding
	if err := unstructured_conversion.DefaultConverter.FromUnstructured(spec.Object, &sicSpec); err != nil {
		return nil, err
	}
	var sicActual sc_v1b1.ServiceBinding
	if err := unstructured_conversion.DefaultConverter.FromUnstructured(actual.Object, &sicActual); err != nil {
		return nil, err
	}

	sicSpec.Spec.ExternalID = sicActual.Spec.ExternalID
	sicSpec.Status = sicActual.Status

	u, err := unstructured_conversion.DefaultConverter.ToUnstructured(&sicSpec)
	if err != nil {
		return nil, err
	}
	return &unstructured.Unstructured{
		Object: u,
	}, nil
}

func scServiceInstanceCleanup(spec, actual *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	var instanceSpec sc_v1b1.ServiceInstance
	if err := unstructured_conversion.DefaultConverter.FromUnstructured(spec.Object, &instanceSpec); err != nil {
		return nil, err
	}
	var instanceActual sc_v1b1.ServiceInstance
	if err := unstructured_conversion.DefaultConverter.FromUnstructured(actual.Object, &instanceActual); err != nil {
		return nil, err
	}

	if instanceActual.Spec.ClusterServiceClassExternalName == instanceSpec.Spec.ClusterServiceClassExternalName {
		instanceSpec.Spec.ClusterServiceClassRef = instanceActual.Spec.ClusterServiceClassRef
	}

	if instanceActual.Spec.ClusterServicePlanExternalName == instanceSpec.Spec.ClusterServicePlanExternalName {
		instanceSpec.Spec.ClusterServicePlanRef = instanceActual.Spec.ClusterServicePlanRef
	}

	instanceSpec.Spec.ExternalID = instanceActual.Spec.ExternalID

	u, err := unstructured_conversion.DefaultConverter.ToUnstructured(&instanceSpec)
	if err != nil {
		return nil, err
	}
	return &unstructured.Unstructured{
		Object: u,
	}, nil
}

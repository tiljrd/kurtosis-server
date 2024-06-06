// cloud.go
package api

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/scheme"
	"text/template"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"
)

type Port struct {
	PortName string
	Port     int32
}

type IngressData struct {
	ServiceName string
	Namespace   string
	Domain      string
	Ports       []Port
}

func loadTemplate(templatePath string) (*template.Template, error) {
	content, err := ioutil.ReadFile(templatePath)
	if err != nil {
		return nil, err
	}

	tmpl, err := template.New("ingress").Parse(string(content))
	if err != nil {
		return nil, err
	}

	return tmpl, nil
}

func createIngress(data IngressData) error {

	fmt.Printf("Creating ingress with the following data: %v", data)
	config, err := clientcmd.BuildConfigFromFlags("", "/home/ubuntu/.kube/config")
	if err != nil {
		return err
	}

	dynClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return err
	}

	// Load the ingress template
	templatePath := "/home/ubuntu/kurtosis-server/internal/api/templates/ingress.tmpl"
	tmpl, err := loadTemplate(templatePath)
	if err != nil {
		return err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return err
	}

	// Print the rendered template for debugging
	fmt.Println("Rendered Ingress YAML:")
	fmt.Println(buf.String())

	// Convert the rendered template to an unstructured object
	ingress := &unstructured.Unstructured{}
	dec := runtime.DecodeInto(scheme.Codecs.UniversalDeserializer(), buf.Bytes(), ingress)
	if dec != nil {
		return err
	}

	fmt.Printf("Creating the following ingress: %v", ingress)

	resource := schema.GroupVersionResource{
		Group:    "networking.k8s.io",
		Version:  "v1",
		Resource: "ingresses",
	}

	namespace := data.Namespace
	name := ingress.GetName()

	// Check if the ingress already exists
	_, err = dynClient.Resource(resource).Namespace(namespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			// Ingress does not exist, create it
			_, err = dynClient.Resource(resource).Namespace(namespace).Create(context.Background(), ingress, metav1.CreateOptions{})
			if err != nil {
				fmt.Printf("Error creating ingress: %v:", err)
				return err
			}
		} else {
			fmt.Printf("Ingress exists or other error: %v", err)
			return err
		}
	}

	return nil
}

func createServiceURLs(data IngressData) []string {
	urls := []string{}
	for _, port := range data.Ports {
		url := fmt.Sprintf("http://%s.%s/%s", data.ServiceName, data.Domain, port.PortName)
		urls = append(urls, url)
	}
	return urls
}

func deleteIngresses(namespace string) error {
	config, err := clientcmd.BuildConfigFromFlags("", "/home/ubuntu/.kube/config")
	if err != nil {
		return err
	}

	dynClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return err
	}

	resource := schema.GroupVersionResource{
		Group:    "networking.k8s.io",
		Version:  "v1",
		Resource: "ingresses",
	}

	// List all ingresses in the namespace
	ingressList, err := dynClient.Resource(resource).Namespace(namespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return err
	}

	// Delete each ingress
	for _, ingress := range ingressList.Items {
		err := dynClient.Resource(resource).Namespace(namespace).Delete(context.Background(), ingress.GetName(), metav1.DeleteOptions{})
		if err != nil {
			return err
		}
	}

	return nil
}

package snapshot

import (
	//	"context"
	"k8s.io/apimachinery/pkg/api/errors"
	"reflect"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	ctrl "sigs.k8s.io/controller-runtime"
	//logf "sigs.k8s.io/controller-runtime/pkg/log"
	//	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appstudiov1alpha1 "github.com/redhat-appstudio/application-service/api/v1alpha1"
	integrationv1alpha1 "github.com/redhat-appstudio/integration-service/api/v1alpha1"
	"github.com/redhat-appstudio/integration-service/gitops"
	appstudioshared "github.com/redhat-appstudio/managed-gitops/appstudio-shared/apis/appstudio.redhat.com/v1alpha1"
	releasev1alpha1 "github.com/redhat-appstudio/release-service/api/v1alpha1"
	tektonv1beta1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	//apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	"github.com/redhat-appstudio/integration-service/helpers"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	//"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	//clientsetscheme "k8s.io/client-go/kubernetes/scheme"
	//"sigs.k8s.io/controller-runtime/pkg/client/fake"
	//"sigs.k8s.io/controller-runtime/pkg/client"
	"github.com/redhat-appstudio/release-service/kcp"
	klog "k8s.io/klog/v2"
)

var _ = Describe("SnapshotController", func() {
	var (
		adapter *Adapter

		testreleasePlan *releasev1alpha1.ReleasePlan
		//	release         *releasev1alpha1.Release
		hasApp                  *appstudiov1alpha1.Application
		hasComp                 *appstudiov1alpha1.Component
		hasComp1                *appstudiov1alpha1.Component
		hasSnapshot             *appstudioshared.ApplicationSnapshot
		testpipelineRun         *tektonv1beta1.PipelineRun
		integrationTestScenario *integrationv1alpha1.IntegrationTestScenario
		//env                     appstudioshared.Environment
	)
	const (
		SampleRepoLink = "https://github.com/devfile-samples/devfile-sample-java-springboot-basic"
	)

	BeforeEach(func() {

		applicationName := "application-sample"

		hasApp = &appstudiov1alpha1.Application{
			ObjectMeta: metav1.ObjectMeta{
				Name:      applicationName,
				Namespace: "default",
			},
			Spec: appstudiov1alpha1.ApplicationSpec{
				DisplayName: "application-sample",
				Description: "This is an example application",
			},
		}

		Expect(k8sClient.Create(ctx, hasApp)).Should(Succeed())

		hasComp = &appstudiov1alpha1.Component{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "component-sample",
				Namespace: "default",
			},
			Spec: appstudiov1alpha1.ComponentSpec{
				ComponentName: "component-sample",
				Application:   applicationName,
				Source: appstudiov1alpha1.ComponentSource{
					ComponentSourceUnion: appstudiov1alpha1.ComponentSourceUnion{
						GitSource: &appstudiov1alpha1.GitSource{
							URL: SampleRepoLink,
						},
					},
				},
			},
		}
		Expect(k8sClient.Create(ctx, hasComp)).Should(Succeed())
		hasComp1 = &appstudiov1alpha1.Component{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "component-sample1",
				Namespace: "default",
			},
			Spec: appstudiov1alpha1.ComponentSpec{
				ComponentName: "component-sample1",
				Application:   applicationName,
				Source: appstudiov1alpha1.ComponentSource{
					ComponentSourceUnion: appstudiov1alpha1.ComponentSourceUnion{
						GitSource: &appstudiov1alpha1.GitSource{
							URL: SampleRepoLink,
						},
					},
				},
			},
		}
		Expect(k8sClient.Create(ctx, hasComp1)).Should(Succeed())
		hasSnapshot = &appstudioshared.ApplicationSnapshot{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "snapshot-sample",
				Namespace: "default",
				Labels: map[string]string{
					gitops.ApplicationSnapshotTypeLabel:      "component",
					gitops.ApplicationSnapshotComponentLabel: "component-sample",
				},
			},
			Spec: appstudioshared.ApplicationSnapshotSpec{
				Application: hasApp.Name,
				Components: []appstudioshared.ApplicationSnapshotComponent{
					{
						Name:           "component-sample",
						ContainerImage: "testimage",
					},
				},
			},
		}
		Expect(k8sClient.Create(ctx, hasSnapshot)).Should(Succeed())

		integrationTestScenario = &integrationv1alpha1.IntegrationTestScenario{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "example-pass",
				Namespace: "default",

				Labels: map[string]string{
					"test.appstudio.openshift.io/optional": "false",
				},
			},
			Spec: integrationv1alpha1.IntegrationTestScenarioSpec{
				Application: "application-sample",
				Bundle:      "quay.io/kpavic/test-bundle:component-pipeline-pass",
				Pipeline:    "component-pipeline-pass",
				Environment: integrationv1alpha1.TestEnvironment{
					Name:   "env_name",
					Type:   "POC",
					Params: []string{},
				},
			},
		}
		Expect(k8sClient.Create(ctx, integrationTestScenario)).Should(Succeed())

		testreleasePlan = &releasev1alpha1.ReleasePlan{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "test-releaseplan-",
				Namespace:    "default",
				Labels: map[string]string{
					releasev1alpha1.AutoReleaseLabel: "true",
				},
			},
			Spec: releasev1alpha1.ReleasePlanSpec{
				Application: hasApp.Name,
				Target: kcp.NamespaceReference{
					Namespace: "default",
					Workspace: "workspace-sample",
				},
			},
		}

		Expect(k8sClient.Create(ctx, testreleasePlan)).Should(Succeed())

		//		release = &releasev1alpha1.Release{
		//			TypeMeta: metav1.TypeMeta{
		//				APIVersion: testApiVersion,
		//				Kind:       "Release",
		//			},
		//			ObjectMeta: metav1.ObjectMeta{
		//				GenerateName: "test-release-",
		//				Namespace:    "default",
		//			},
		//			Spec: releasev1alpha1.ReleaseSpec{
		//				ApplicationSnapshot: "test-snapshot",
		//				ReleasePlan:         testreleasePlan.GetName(),
		//			},
		//		}
		//		Expect(k8sClient.Create(ctx, release)).Should(Succeed())

		testpipelineRun = &tektonv1beta1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "build-pipelinerun" + "-",
				Namespace:    "default",
				Labels: map[string]string{
					"pipelines.appstudio.openshift.io/type":  "build",
					"pipelines.openshift.io/used-by":         "build-cloud",
					"pipelines.openshift.io/runtime":         "nodejs",
					"pipelines.openshift.io/strategy":        "s2i",
					"build.appstudio.openshift.io/component": "component-sample",
				},
				Annotations: map[string]string{
					"appstudio.redhat.com/updateComponentOnSuccess": "false",
				},
			},
			Spec: tektonv1beta1.PipelineRunSpec{
				PipelineRef: &tektonv1beta1.PipelineRef{
					Name:   "build-pipeline-pass",
					Bundle: "quay.io/kpavic/test-bundle:build-pipeline-pass",
				},
				Params: []tektonv1beta1.Param{
					{
						Name: "output-image",
						Value: tektonv1beta1.ArrayOrString{
							Type:      "string",
							StringVal: "quay.io/redhat-appstudio/sample-image",
						},
					},
				},
			},
			//		Status: tektonv1beta1.PipelineRunStatus{
			//			PipelineRunStatusFields: tektonv1beta1.PipelineRunStatusFields{
			//				TaskRuns: map[string]*tektonv1beta1.PipelineRunTaskRunStatus{
			//					"index1": &tektonv1beta1.PipelineRunTaskRunStatus{
			//						PipelineTaskName: "build-container",
			//						Status: &tektonv1beta1.TaskRunStatus{
			//							TaskRunStatusFields: tektonv1beta1.TaskRunStatusFields{
			//								TaskRunResults: []tektonv1beta1.TaskRunResult{
			//									{
			//										Name:  "IMAGE_DIGEST",
			//										Value: "image_digest_value",
			//									},
			//								},
			//							},
			//						},
			//					},
			//				},
			//				},
			//},
		}
		Expect(k8sClient.Create(ctx, testpipelineRun)).Should(Succeed())

		//		env := appstudioshared.Environment{
		//			ObjectMeta: metav1.ObjectMeta{
		//				Name:      "env_name",
		//				Namespace: "default",
		//			},
		//			Spec: appstudioshared.EnvironmentSpec{
		//				Type:               "POC",
		//				DisplayName:        "my-environment",
		//				DeploymentStrategy: appstudioshared.DeploymentStrategy_Manual,
		//				ParentEnvironment:  "",
		//				Tags:               []string{},
		//				Configuration:      appstudioshared.EnvironmentConfiguration{},
		////				UnstableConfigurationFields: &appstudioshared.UnstableEnvironmentConfiguration{
		//					KubernetesClusterCredentials: appstudioshared.KubernetesClusterCredentials{
		//						TargetNamespace: "my-target-namespace",
		//						APIURL:          "https://my-api-url",
		//						ClusterCredentialsSecret: secret.Name,
		//					},
		//				},
		////			},
		//		}
		////		Expect(k8sClient.Create(ctx, env)).Should(Succeed())

		Eventually(func() error {
			err := k8sClient.Get(ctx, types.NamespacedName{
				Name:      hasSnapshot.Name,
				Namespace: "default",
			}, hasSnapshot)
			return err
		}, time.Second*10).ShouldNot(HaveOccurred())

		adapter = NewAdapter(hasSnapshot, hasApp, hasComp, ctrl.Log, k8sClient, ctx)
		//adapter.targetContext = adapter.context
		Expect(reflect.TypeOf(adapter)).To(Equal(reflect.TypeOf(&Adapter{})))

	})
	AfterEach(func() {
		err := k8sClient.Delete(ctx, hasApp)
		Expect(err == nil || errors.IsNotFound(err)).To(BeTrue())
		err = k8sClient.Delete(ctx, hasComp)
		Expect(err == nil || errors.IsNotFound(err)).To(BeTrue())
		err = k8sClient.Delete(ctx, hasComp1)
		Expect(err == nil || errors.IsNotFound(err)).To(BeTrue())
		err = k8sClient.Delete(ctx, hasSnapshot)
		Expect(err == nil || errors.IsNotFound(err)).To(BeTrue())
		err = k8sClient.Delete(ctx, testpipelineRun)
		Expect(err == nil || errors.IsNotFound(err)).To(BeTrue())
		err = k8sClient.Delete(ctx, integrationTestScenario)
		Expect(err == nil || errors.IsNotFound(err)).To(BeTrue())
	})
	It("can create a new Adapter instance", func() {
		Expect(reflect.TypeOf(NewAdapter(hasSnapshot, hasApp, hasComp, ctrl.Log, k8sClient, ctx))).To(Equal(reflect.TypeOf(&Adapter{})))
	})
	It("ensures the Applicationcomponents can be found ", func() {
		applicationComponents, err := adapter.getAllApplicationComponents(hasApp)
		Expect(err == nil).To(BeTrue())
		Expect(applicationComponents != nil).To(BeTrue())
	})
	It("ensures the integrationTestPipelines are created", func() {
		result, err := adapter.EnsureAllIntegrationTestPipelinesExist()
		Expect(err).ShouldNot(HaveOccurred())
		Expect(result.CancelRequest).To(BeFalse())

		requiredIntegrationTestScenarios, err := helpers.GetRequiredIntegrationTestScenariosForApplication(k8sClient, ctx, hasApp)
		Expect(requiredIntegrationTestScenarios != nil && err == nil).To(BeTrue())
		for _, requiredIntegrationTestScenario := range *requiredIntegrationTestScenarios {
			klog.Infof("The Application in requiredIntegrationTestScenario", requiredIntegrationTestScenario.Spec.Application)
			//		Expect(k8sClient.Delete(ctx, requiredIntegrationTestScenario)).Should(Succeed())
		}
		//pipelineRun, err := adapter.getReleasePipelineRun()
		//		Expect(pipelineRun != nil && err == nil).To(BeTrue())
		//		Expect(k8sClient.Delete(ctx, pipelineRun)).Should(Succeed())
	})

	It("ensures all Releases exists", func() {
		result, err := adapter.EnsureAllReleasesExist()
		Expect(err).ShouldNot(HaveOccurred())
		Expect(result.CancelRequest).To(BeFalse())
		//		Expect(k8sClient.Delete(ctx, pipelineRun)).Should(Succeed())
	})

	It("ensures global Component Image updated", func() {
		result, err := adapter.EnsureGlobalComponentImageUpdated()
		Expect(err).ShouldNot(HaveOccurred())
		Expect(result.CancelRequest).To(BeFalse())
	})

	//	It("ensures applicationsnapshot environmentBinding exist", func() {
	//		result, err := adapter.EnsureApplicationSnapshotEnvironmentBindingExist()
	//		Expect(err).ShouldNot(HaveOccurred())
	//		Expect(result.CancelRequest).To(BeFalse())
	//	})

})

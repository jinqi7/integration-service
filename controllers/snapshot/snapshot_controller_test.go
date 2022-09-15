package snapshot

import (
	//	"context"
	"k8s.io/apimachinery/pkg/api/errors"
	"reflect"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appstudiov1alpha1 "github.com/redhat-appstudio/application-service/api/v1alpha1"
	integrationv1alpha1 "github.com/redhat-appstudio/integration-service/api/v1alpha1"
	"github.com/redhat-appstudio/integration-service/gitops"
	appstudioshared "github.com/redhat-appstudio/managed-gitops/appstudio-shared/apis/appstudio.redhat.com/v1alpha1"
	releasev1alpha1 "github.com/redhat-appstudio/release-service/api/v1alpha1"
	tektonv1beta1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	//apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientsetscheme "k8s.io/client-go/kubernetes/scheme"
	//"sigs.k8s.io/controller-runtime/pkg/client/fake"
	//"sigs.k8s.io/controller-runtime/pkg/client"
	"github.com/redhat-appstudio/release-service/kcp"
	klog "k8s.io/klog/v2"
)

var _ = Describe("SnapshotController", func() {
	var (
		manager         ctrl.Manager
		reconciler      *Reconciler
		scheme          runtime.Scheme
		req             ctrl.Request
		testreleasePlan *releasev1alpha1.ReleasePlan
		//	release         *releasev1alpha1.Release
		//		releasePlan     *releasev1alpha1.ReleasePlan
		hasApp                  *appstudiov1alpha1.Application
		hasComp                 *appstudiov1alpha1.Component
		hasSnapshot             *appstudioshared.ApplicationSnapshot
		testpipelineRun         *tektonv1beta1.PipelineRun
		integrationTestScenario *integrationv1alpha1.IntegrationTestScenario
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
			},
			Spec: integrationv1alpha1.IntegrationTestScenarioSpec{
				Application: "application-sample",
				Bundle:      "quay.io/kpavic/test-bundle:component-pipeline-pass",
				Pipeline:    "component-pipeline-pass",
				Environment: integrationv1alpha1.TestEnvironment{
					Name:   "env_name",
					Type:   "root",
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
				GenerateName: "prefix" + "-",
				Namespace:    "default",
			},
			Spec: tektonv1beta1.PipelineRunSpec{
				Params: []tektonv1beta1.Param{
					{
						Name: "output-image",
						Value: tektonv1beta1.ArrayOrString{
							Type:      "string",
							StringVal: "testimage",
						},
					},
				},
			},
			Status: tektonv1beta1.PipelineRunStatus{
				PipelineRunStatusFields: tektonv1beta1.PipelineRunStatusFields{
					TaskRuns: map[string]*tektonv1beta1.PipelineRunTaskRunStatus{
						"index1": &tektonv1beta1.PipelineRunTaskRunStatus{
							PipelineTaskName: "build-container",
							Status: &tektonv1beta1.TaskRunStatus{
								TaskRunStatusFields: tektonv1beta1.TaskRunStatusFields{
									TaskRunResults: []tektonv1beta1.TaskRunResult{
										{
											Name:  "IMAGE_DIGEST",
											Value: "image_digest_value",
										},
									},
								},
							},
						},
					},
				},
			},
		}
		Expect(k8sClient.Create(ctx, testpipelineRun)).Should(Succeed())

		req = ctrl.Request{
			NamespacedName: types.NamespacedName{
				Namespace: "default",
				Name:      hasSnapshot.Name,
			},
		}

		webhookInstallOptions := &testEnv.WebhookInstallOptions

		klog.Info(webhookInstallOptions.LocalServingHost)
		klog.Info(webhookInstallOptions.LocalServingPort)
		klog.Info(webhookInstallOptions.LocalServingCertDir)

		var err error
		manager, err = ctrl.NewManager(cfg, ctrl.Options{
			Scheme:             clientsetscheme.Scheme,
			Host:               webhookInstallOptions.LocalServingHost,
			Port:               webhookInstallOptions.LocalServingPort,
			CertDir:            webhookInstallOptions.LocalServingCertDir,
			MetricsBindAddress: "0", // this disables metrics
			LeaderElection:     false,
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(err).To(BeNil())

		reconciler = NewSnapshotReconciler(k8sClient, &logf.Log, &scheme)
	})
	AfterEach(func() {
		err := k8sClient.Delete(ctx, hasApp)
		Expect(err == nil || errors.IsNotFound(err)).To(BeTrue())
		err = k8sClient.Delete(ctx, hasComp)
		Expect(err == nil || errors.IsNotFound(err)).To(BeTrue())
		err = k8sClient.Delete(ctx, hasSnapshot)
		Expect(err == nil || errors.IsNotFound(err)).To(BeTrue())
		err = k8sClient.Delete(ctx, testpipelineRun)
		Expect(err == nil || errors.IsNotFound(err)).To(BeTrue())
		err = k8sClient.Delete(ctx, integrationTestScenario)
		Expect(err == nil || errors.IsNotFound(err)).To(BeTrue())
	})

	It("can create and return a new Reconciler object", func() {
		Expect(reflect.TypeOf(reconciler)).To(Equal(reflect.TypeOf(&Reconciler{})))
		klog.Info("Test First Logic")
	})

	It("should reconcile using the ReconcileHandler", func() {
		adapter := NewAdapter(hasSnapshot, hasApp, hasComp, ctrl.Log, k8sClient, ctx)
		result, err := reconciler.ReconcileHandler(adapter)
		Expect(reflect.TypeOf(result)).To(Equal(reflect.TypeOf(reconcile.Result{})))
		Expect(err).To(BeNil())
	})

	It("can fail when Reconcile fails to prepare the adapter when snapshot is not found", func() {
		Expect(k8sClient.Delete(ctx, hasSnapshot)).Should(Succeed())
		Eventually(func() error {
			_, err := reconciler.Reconcile(ctx, req)
			return err
		}).Should(BeNil())
	})

	It("can Reconcile function prepare the adapter and return the result of the reconcile handling operation", func() {
		result, err := reconciler.Reconcile(ctx, req)
		Expect(reflect.TypeOf(result)).To(Equal(reflect.TypeOf(reconcile.Result{})))
		Expect(err).To(BeNil())
	})

	It("can setup the cache by adding a new index field to search for ReleasePlanAdmissions", func() {
		err := setupCache(manager)
		Expect(err).ToNot(HaveOccurred())
	})

	It("can setup a new controller manager with the given reconciler", func() {
		err := setupControllerWithManager(manager, reconciler)
		Expect(err).NotTo(HaveOccurred())
	})

	It("can setup a new Controller manager and start it", func() {
		err := SetupController(manager, &ctrl.Log)
		Expect(err).To(BeNil())
		go func() {
			defer GinkgoRecover()
			err = manager.Start(ctx)
			Expect(err).NotTo(HaveOccurred())
		}()
	})

})

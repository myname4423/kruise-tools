/*
Copyright 2021 The Kruise Authors.
Copyright 2016 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package rollout

import (
	"fmt"

	internalapi "github.com/openkruise/kruise-tools/pkg/api"
	internalpolymorphichelpers "github.com/openkruise/kruise-tools/pkg/internal/polymorphichelpers"
	kruiserolloutsv1apha1 "github.com/openkruise/rollouts/api/v1alpha1"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/printers"
	"k8s.io/cli-runtime/pkg/resource"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/scheme"
	"k8s.io/kubectl/pkg/util/i18n"
	"k8s.io/kubectl/pkg/util/templates"
)

// UndoOptions is the start of the data required to perform the operation.  As new fields are added, add them here instead of
// referencing the cmd.Flags()
type UndoOptions struct {
	PrintFlags *genericclioptions.PrintFlags
	ToPrinter  func(string) (printers.ResourcePrinter, error)

	Builder          func() *resource.Builder
	ToRevision       int64
	DryRunStrategy   cmdutil.DryRunStrategy
	DryRunVerifier   *resource.DryRunVerifier
	Resources        []string
	Namespace        string
	EnforceNamespace bool
	RESTClientGetter genericclioptions.RESTClientGetter

	resource.FilenameOptions
	genericclioptions.IOStreams
}

var (
	undoLong = templates.LongDesc(`
		Rollback to a previous rollout.`)

	undoExample = templates.Examples(`
		# Rollback to the previous cloneset
		kubectl-kruise rollout undo cloneset/abc

		# Rollback to the previous Advanced StatefulSet
		kubectl-kruise rollout undo asts/abc

		# Rollback to daemonset revision 3
		kubectl-kruise rollout undo daemonset/abc --to-revision=3

		# Rollback to the previous deployment with dry-run
		kubectl-kruise rollout undo --dry-run=server deployment/abc
		
		# Rollback to workload via rollout api object
		kubectl-kruise rollout undo rollout/abc`)
)

// NewRolloutUndoOptions returns an initialized UndoOptions instance
func NewRolloutUndoOptions(streams genericclioptions.IOStreams) *UndoOptions {
	return &UndoOptions{
		PrintFlags: genericclioptions.NewPrintFlags("rolled back").WithTypeSetter(internalapi.GetScheme()),
		IOStreams:  streams,
		ToRevision: int64(0),
	}
}

// NewCmdRolloutUndo returns a Command instance for the 'rollout undo' sub command
func NewCmdRolloutUndo(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewRolloutUndoOptions(streams)

	validArgs := []string{"deployment", "daemonset", "statefulset", "cloneset", "advanced statefulset", "rollout"}

	cmd := &cobra.Command{
		Use:                   "undo (TYPE NAME | TYPE/NAME) [flags]",
		DisableFlagsInUseLine: true,
		Short:                 i18n.T("Undo a previous rollout"),
		Long:                  undoLong,
		Example:               undoExample,
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Complete(f, cmd, args))
			cmdutil.CheckErr(o.Validate())
			cmdutil.CheckErr(o.RunUndo())
		},
		ValidArgs: validArgs,
	}

	cmd.Flags().Int64Var(&o.ToRevision, "to-revision", o.ToRevision, "The revision to rollback to. Default to 0 (last revision).")
	usage := "identifying the resource to get from a server."
	cmdutil.AddFilenameOptionFlags(cmd, &o.FilenameOptions, usage)
	cmdutil.AddDryRunFlag(cmd)
	o.PrintFlags.AddFlags(cmd)
	return cmd
}

// Complete completes all the required options
func (o *UndoOptions) Complete(f cmdutil.Factory, cmd *cobra.Command, args []string) error {
	o.Resources = args
	var err error
	o.DryRunStrategy, err = cmdutil.GetDryRunStrategy(cmd)
	if err != nil {
		return err
	}
	dynamicClient, err := f.DynamicClient()
	if err != nil {
		return err
	}
	discoveryClient, err := f.ToDiscoveryClient()
	if err != nil {
		return err
	}
	o.DryRunVerifier = resource.NewDryRunVerifier(dynamicClient, discoveryClient)

	if o.Namespace, o.EnforceNamespace, err = f.ToRawKubeConfigLoader().Namespace(); err != nil {
		return err
	}

	o.ToPrinter = func(operation string) (printers.ResourcePrinter, error) {
		o.PrintFlags.NamePrintFlags.Operation = operation
		cmdutil.PrintFlagsWithDryRunStrategy(o.PrintFlags, o.DryRunStrategy)
		return o.PrintFlags.ToPrinter()
	}

	o.RESTClientGetter = f
	o.Builder = f.NewBuilder

	return err
}

func (o *UndoOptions) Validate() error {
	if len(o.Resources) == 0 && cmdutil.IsFilenameSliceEmpty(o.Filenames, o.Kustomize) {
		return fmt.Errorf("required resource not specified")
	}
	return nil
}

// func (o *UndoOptions) CheckRollout() error {
// 	r := o.Builder().
// 		WithScheme(internalapi.GetScheme(), scheme.Scheme.PrioritizedVersionsAllGroups()...).
// 		NamespaceParam(o.Namespace).DefaultNamespace().
// 		FilenameParam(o.EnforceNamespace, &o.FilenameOptions).
// 		ResourceTypeOrNameArgs(true, o.Resources...). //Set Resources
// 		ContinueOnError().
// 		Latest(). // Latest will fetch the latest copy of any objects loaded from URLs or files from the server.
// 		Flatten().
// 		Do() //Do returns a Result object with a Visitor for the resources
// 	if err := r.Err(); err != nil {
// 		return err
// 	}

// 	infos, err := r.Infos()
// 	if err != nil {
// 		return err
// 	}
// 	var RefResources []string
// 	for _, info := range infos {
// 		obj := info.Object
// 		ro, ok := obj.(*kruiserolloutsv1apha1.Rollout)
// 		if !ok {
// 			continue
// 		}
// 		ResourceTypeAndName := ro.Spec.ObjectRef.WorkloadRef.Kind + "/" + ro.Spec.ObjectRef.WorkloadRef.Name
// 		printer, err := o.ToPrinter(fmt.Sprintf("refers to %s", ResourceTypeAndName))
// 		if err != nil {
// 			return err
// 		}
// 		err = printer.PrintObj(info.Object, o.Out)
// 		if err != nil {
// 			return err
// 		}
// 		RefResources = append(RefResources, ResourceTypeAndName)
// 	}
// 	//REVIEW - is deduplication needed?
// 	o.Resources = append(o.Resources, RefResources...)
// 	return nil
// }

// RunUndo performs the execution of 'rollout undo' sub command
func (o *UndoOptions) RunUndo() error {
	r := o.Builder().
		WithScheme(internalapi.GetScheme(), scheme.Scheme.PrioritizedVersionsAllGroups()...).
		NamespaceParam(o.Namespace).DefaultNamespace().
		FilenameParam(o.EnforceNamespace, &o.FilenameOptions).
		ResourceTypeOrNameArgs(true, o.Resources...).
		ContinueOnError().
		Latest().
		Flatten().Do()
	if err := r.Err(); err != nil {
		return err
	}

	// perform undo logic here
	undoFunc := func(info *resource.Info, err error) error {
		if err != nil {
			return err
		}
		rollbacker, err := internalpolymorphichelpers.RollbackerFn(o.RESTClientGetter, info.ResourceMapping())
		if err != nil {
			return err
		}

		if o.DryRunStrategy == cmdutil.DryRunServer {
			if err := o.DryRunVerifier.HasSupport(info.Mapping.GroupVersionKind); err != nil {
				return err
			}
		}
		result, err := rollbacker.Rollback(info.Object, nil, o.ToRevision, o.DryRunStrategy)
		if err != nil {
			return err
		}

		printer, err := o.ToPrinter(result)
		if err != nil {
			return err
		}

		return printer.PrintObj(info.Object, o.Out)
	}

	var refResources []string
	// When multiple rollout objects specified within the arguments reference a single workload (inclusive of the workload itself),
	// performing multiple undo operations on the workload in a single command is not smart. Such an action could
	// lead to confusion and yield unintended consequences. Consequently, undo operations in this context are disallowed.
	// Should such a scenario occur, the system will report an error and only the first argument that points to the workload will be executed.
	deDuplica := make(map[string]struct{})

	err := r.Visit(func(info *resource.Info, err error) error {
		if err != nil {
			return err
		}

		if info.Mapping.GroupVersionKind.Group == "rollouts.kruise.io" && info.Mapping.GroupVersionKind.Kind == "Rollout" {
			obj := info.Object
			if obj == nil {
				fmt.Println("Rollout object not found")
				return fmt.Errorf("Rollout object not found")
			}
			ro, ok := obj.(*kruiserolloutsv1apha1.Rollout)
			if !ok {
				fmt.Println("unsupported version of Rollout")
				return fmt.Errorf("unsupported version of Rollout")
			}
			workloadRef := ro.Spec.ObjectRef.WorkloadRef
			gv, err := schema.ParseGroupVersion(workloadRef.APIVersion)
			if err != nil {
				return err
			}
			gvk := &schema.GroupVersionKind{Group: gv.Group, Version: gv.Version, Kind: workloadRef.Kind}
			deDuplicaKey := gvk.String() + workloadRef.Name
			if _, ok := deDuplica[deDuplicaKey]; ok {
				fmt.Println("出现重复了，不允许在一次rollout undo命令中对同一个对象多次undo")
				return fmt.Errorf("出现重复了，不允许在一次rollout undo命令中对同一个对象多次undo")
			}

			resourceIdentifier := workloadRef.Kind + "." + gv.Version + "." + gv.Group + "/" + workloadRef.Name
			printer, err := o.ToPrinter(fmt.Sprintf("references to %s", resourceIdentifier))
			if err != nil {
				return err
			}
			err = printer.PrintObj(info.Object, o.Out)
			if err != nil {
				return err
			}
			deDuplica[deDuplicaKey] = struct{}{}
			refResources = append(refResources, resourceIdentifier)
			return nil
		} else {
			deDuplicaKey := info.Mapping.GroupVersionKind.String() + info.Name
			//去除本身的重复
			if _, ok := deDuplica[deDuplicaKey]; ok {
				fmt.Println("出现重复了，不允许在一次rollout undo命令中对同一个对象多次undo")
				return fmt.Errorf("出现重复了，不允许在一次rollout undo命令中对同一个对象多次undo")
			}
			deDuplica[deDuplicaKey] = struct{}{}
		}

		return undoFunc(info, nil)
	})
	if err != nil {
		//TODO - 如何集合错误？拼接？
		// return err
	}

	if len(refResources) < 1 {
		return nil
	}

	//REVIEW - 访问refered workload， 如果这样有问题的话就从头搭建一个builder就行了
	r2 := o.Builder().
		WithScheme(internalapi.GetScheme(), scheme.Scheme.PrioritizedVersionsAllGroups()...).
		NamespaceParam(o.Namespace).DefaultNamespace().
		FilenameParam(o.EnforceNamespace, &o.FilenameOptions).
		ResourceTypeOrNameArgs(true, refResources...).
		ContinueOnError().
		Latest().
		Flatten().Do()
	if err2 := r2.Err(); err2 != nil {
		return err2
	}
	err2 := r2.Visit(undoFunc)

	return fmt.Errorf(err.Error() + "\n" + err2.Error())
}

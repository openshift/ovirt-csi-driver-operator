package config

// Config is configuration of the CSI Driver operator.
type Config struct {
	// Container images used for deploying CSI driver and sidecar components.
	Images CSIDeploymentContainerImages

	// Selector of nodes where Deployment with controller components (provisioner, attacher) can run.
	// When nil, no selector will be set in the Deployment.
	InfrastructureNodeSelector map[string]string

	// Number of replicas of Deployment with controller components.
	DeploymentReplicas int32

	// Name of cluster role to bind to ServiceAccount that runs all pods with drivers. This role allows to run
	// provisioner, attacher and driver registrar, i.e. read/modify PV, PVC, Node, VolumeAttachment and whatnot in
	// *any* namespace.
	// TODO: should there be multiple ClusterRoles, separate one for provisioner, attacher and driver registrar?
	// In addition, some of them may require variants (e.g. provisioner without access to all secrets and / or attacher
	// without access to all secrets)
	ClusterRoleName string

	// Name of cluster role to bind to ServiceAccount that runs all pods with drivers. This role allows attacher and
	// provisioner to run leader election. It will be bound to the ServiceAccount using RoleBind, i.e. leader election
	// will be possible only in the namespace where the drivers run.
	LeaderElectionClusterRoleName string
}

// CSIDeploymentContainerImages specifies custom sidecar container image names.
type CSIDeploymentContainerImages struct {
	// CSIDriver is the name of the CSI driver container image.
	CSIDriver string

	// AttacherImage is the name of the CSI Attacher sidecar container image.
	AttacherImage string

	// ProvisionerImage is the name of the CSI Provisioner sidecar container image.
	ProvisionerImage string

	// DriverRegistrarImage is the name of the CSI Driver Registrar sidecar container image.
	DriverRegistrarImage string

	// LivenessProbeImage is the name of CSI Liveness Probe sidecar container image.
	LivenessProbeImage string

	// ResizerImage is the name of the CSI Resizer sidecar container image.
	ResizerImage string

	// SnapshoterImage is the name of the CSI Snapshoter sidecar container image.
	SnapshoterImage string
}

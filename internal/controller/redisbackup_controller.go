func (r *RedisBackupReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqLogger := log.FromContext(ctx)
	reqLogger.Info("Reconciling RedisBackup")

	redisBackup := &redisv1alpha1.RedisBackup{}
	err := r.Get(ctx, req.NamespacedName, redisBackup)
	if err != nil {
		if errors.IsNotFound(err) {
			reqLogger.Info("RedisBackup resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}

		reqLogger.Error(err, "Failed to get RedisBackup")
		return ctrl.Result{}, err
	}

	reqLogger.Info("Successfully fetched RedisBackup file!",
		"TargetCluster", redisBackup.Spec.RedisClusterName,
		"S3Bucket", redisBackup.Spec.S3Bucket)

	return ctrl.Result{}, nil
}

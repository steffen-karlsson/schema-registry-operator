stateDiagram-v2
    direction LR
    %% Hack to mack them separate
    classDef reconcile fill:#7FFFD4

    reconcile1:::reconcile: Reconcile
    reconcile2:::reconcile: Reconcile
    reconcile3:::reconcile: Reconcile
    reconcile4:::reconcile: Reconcile (x min)
    exist: Exist
    state is_exist <<choice>>

    schema_registry_label: Has Schema Registry Label
    state has_schema_registry_label <<choice>>

    schema_registry_exist: Schema Registry Exist
    state is_schema_registry_exist <<choice>>

    marked_to_be_deleted: Marked To Be Deleted
    state is_marked_to_be_deleted <<choice>>

    new_object: Is new object
    state is_new_object <<choice>>

    [*] -->  exist
    exist -->  is_exist
    is_exist --> reconcile1: No

    is_exist --> schema_registry_label
    schema_registry_label --> has_schema_registry_label
    has_schema_registry_label --> reconcile2: No

    has_schema_registry_label --> schema_registry_exist
    schema_registry_exist --> is_schema_registry_exist
    is_schema_registry_exist --> reconcile3: No

    is_schema_registry_exist --> marked_to_be_deleted: Yes
    marked_to_be_deleted --> is_marked_to_be_deleted
    is_marked_to_be_deleted --> DeleteSchemaReconciler: Yes

    is_marked_to_be_deleted --> new_object: No
    new_object --> is_new_object
    is_new_object --> CreateSchemaReconciler: Yes

    is_new_object --> UpdateSchemaReconciler: No


    state DeleteSchemaReconciler {
        delete_schema: Delete Schema
        delete_finalizer: Delete Finalizer
        update_resource_delete: Update Resource

        delete_schema --> delete_finalizer
        delete_finalizer --> update_resource_delete
    }

    state CreateSchemaReconciler {
        add_finalizer: Add Finalizer
        subject_unique: Is Subject Unique
        state is_subject_unique <<choice>>
        update_resource_create: Update Resource

        add_finalizer --> subject_unique
        subject_unique --> is_subject_unique
        is_subject_unique --> [*]: No
        is_subject_unique --> update_resource_create
    }

    state UpdateSchemaReconciler {
        update_schema: Update Schema
        apply_compatibilty_level: Apply Compatibility Level
        update_status: Update Status

        update_schema --> apply_compatibilty_level
        apply_compatibilty_level --> update_status
    }

    DeleteSchemaReconciler --> reconcile4
    CreateSchemaReconciler --> reconcile4
    UpdateSchemaReconciler --> reconcile4

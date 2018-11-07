#!/usr/bin/env bash

# this just deploys everything under /manifests,
# but tries to space them out a bit to avoid errors.
# in the end, it creates a custom resource to kick
# the operator into action

# necessary if doing dev locally on a < 4.0.0 cluster
CLUSTER_OPERATOR_CRD_FILE="./examples/crd-clusteroperator.yaml"
echo "creating ${CLUSTER_OPERATOR_CRD_FILE}"
oc create -f "${CLUSTER_OPERATOR_CRD_FILE}"

# examples/cr.yaml is not necessary as the operator will create
# an instance of a "console" by default.
# use it if customization is desired.

for FILE in `find ./manifests -name '00-*'`
do
    echo "creating ${FILE}"
  oc create -f $FILE
done

sleep 1

for FILE in `find ./manifests -name '01-*'`
do
    echo "creating ${FILE}"
  oc create -f $FILE
done

sleep 2

for FILE in `find ./manifests -name '02-*'`
do
  echo "creating ${FILE}"
  oc create -f $FILE
done

sleep 1

for FILE in `find ./manifests -name '03-*'`
do
  echo "creating ${FILE}"
  oc create -f $FILE
done

sleep 1

for FILE in `find ./manifests -name '04-*'`
do
  echo "creating ${FILE}"
  oc create -f $FILE
done

sleep 1

# Don't deploy the operator in `manifests`
# instead, we will instantiate the operator locally
#
#for FILE in `find ./manifests -name '05-*'`
#do
#  echo "creating ${FILE}"
#  oc create -f $FILE
#done
IMAGE=docker.io/openshift/origin-console:latest \
    console operator \
    --kubeconfig $HOME/.kube/config \
    --config examples/config.yaml \
    --v 4

echo "TODO: support --create-default-console again!"

# TODO: GET BACK TO THIS:
#IMAGE=docker.io/openshift/origin-console:latest \
#    console operator \
#    --kubeconfig $HOME/.kube/config \
#    --config examples/config.yaml \
#    --create-default-console \
#    --v 4



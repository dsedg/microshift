FROM quay.io/fedora/fedora:41 as builder

RUN curl https://mirror.openshift.com/pub/openshift-v4/clients/oc/latest/linux/oc.tar.gz -s -L | tar xvz -C /usr/bin oc 

RUN dnf install -y yq gettext python3-pyyaml gcc git golang cockpit make jq selinux-policy-devel rpm-build jq bash-completion avahi-tools createrepo

WORKDIR /src/

COPY okd/src/install_go.sh .

RUN sh install_go.sh

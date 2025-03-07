# Home

## Overview

Kernel Module Management (KMM) is an Openshift operator that manages, builds, signs and deploys out-of-tree kernel modules and device plugins on Openshift clusters.

KMM adds a new `Module` CRD which describes the desired state of an out-of-tree kernel module and its associated device plugin. `Module` resources contain fields that configure how to load the module, associates ModuleLoader images with kernel versions, and optionally instructions to build and sign modules for specific kernel versions.
KMM is designed to accomodate several kernel versions at once for any kernel module, allowing for seamless node upgrades and reduced application downtime.

## Installation Guide 

Check the [Install](documentation/install.md) section for instructions.

## Configuration

wgmesh allows for a `config` parameter which points to a yaml configuration file. If given, config file is read and command line parameters are applied on top of it.

Example:

```yaml
mesh-name: somemesh
node-name: mynodename
bootstrap:
    mesh-cidr-range: 10.233.0.0/16
```

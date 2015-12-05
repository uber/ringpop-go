Internally, Ringpop and its subpackages should not depend concretely upon one
another. They should depend on interface types. However, interfaces of subpackages
should not be made public. Herein lies the interfaces of 1st party dependencies.

WARNING: Though keeping deps/* internal prevents 3rd parties from importing them,
it does not prevent us from exporting them unintentionally. Be careful not to make
any of the internal types a part of an exported API.

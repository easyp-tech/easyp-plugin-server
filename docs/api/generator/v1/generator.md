# Protocol Documentation
<a name="top"></a>

## Table of Contents

- [api/generator/v1/generator.proto](#api-generator-v1-generator-proto)
  - **Services**
    - [ServiceAPI](#api-generator-v1-serviceapi)
  - **Messages**
    - [GenerateCodeRequest](#api-generator-v1-generatecoderequest)
    - [GenerateCodeResponse](#api-generator-v1-generatecoderesponse)
    - [PluginsRequest](#api-generator-v1-pluginsrequest)
    - [PluginsResponse](#api-generator-v1-pluginsresponse)
    - [PluginInfo](#api-generator-v1-plugininfo)

<a name="api-generator-v1-generator-proto"></a>
<p align="right"><a href="#top">Top</a></p>

## api/generator/v1/generator.proto

**Package:** `api.generator.v1`

<a name="api-generator-v1-serviceapi"></a>

## ServiceAPI

EasyP Code Generation Service

This service provides a centralized API for executing protobuf/gRPC code generation plugins.
Plugins run as isolated Docker containers, ensuring version consistency across development teams.

## Benefits

- **Version Control**: All developers use the same plugin versions
- **Zero Setup**: No local plugin installation required
- **Security**: Plugins run in sandboxed containers with resource limits
- **Auditability**: Centralized logging of all generation requests

## Quick Start

1. Call `Plugins` to list available plugins
2. Build a `CodeGeneratorRequest` with your proto files
3. Call `GenerateCode` with the plugin name and request
4. Process the `CodeGeneratorResponse` with generated files

## Plugin Naming Convention

Plugins are identified using the format: `<group>/<name>:<version>`

Examples:
- `protocolbuffers/go:v1.36.10` — Official Go protobuf plugin
- `grpc/go:v1.5.1` — Official gRPC Go plugin
- `grpc-ecosystem/gateway:v2.27.3` — gRPC-Gateway plugin

Use `latest` as version to get the most recent version:
- `protocolbuffers/go:latest`

### Methods Overview

| Method | Type | HTTP | Description |
| ------ | ---- | ---- | ----------- |
| [GenerateCode](#api-generator-v1-serviceapi-generatecode) | ➡️ Unary | — | Generate code using a specified plugin.  This method execute... |
| [Plugins](#api-generator-v1-serviceapi-plugins) | ➡️ Unary | — | List available plugins.  Returns a list of all plugins regis... |

<a name="api-generator-v1-serviceapi-generatecode"></a>

### GenerateCode

```protobuf
rpc GenerateCode([GenerateCodeRequest](#api-generator-v1-generatecoderequest)) returns ([GenerateCodeResponse](#api-generator-v1-generatecoderesponse))
```

Generate code using a specified plugin.

This method executes a protobuf code generation plugin and returns the generated files.
The plugin runs in an isolated Docker container with the following default limits:

- **Network**: Disabled (no external access)
- **Memory**: 128MB
- **CPU**: 1.0 core

## Error Codes

| Code | Description |
|------|-------------|
| `NOT_FOUND` | Plugin not found in registry |
| `INVALID_ARGUMENT` | Invalid plugin name format |
| `INTERNAL` | Plugin execution failed |
| `DEADLINE_EXCEEDED` | Plugin execution timeout |

#### Request Example

```json
{
  "codeGeneratorRequest": {
    "compilerVersion": {
      "major": 0,
      "minor": 0,
      "patch": 0,
      "suffix": "string"
    },
    "fileToGenerate": [
      "string"
    ],
    "parameter": "string",
    "protoFile": [
      {
        "dependency": [
          "string"
        ],
        "edition": "Edition_VALUE",
        "enumType": [
          {
            "name": "string",
            "options": {
              "allowAlias": true,
              "deprecated": true,
              "deprecatedLegacyJsonFieldConflicts": true,
              "features": {},
              "uninterpretedOption": []
            },
            "reservedName": [
              "string"
            ],
            "reservedRange": [
              {
                "end": 0,
                "start": 0
              }
            ],
            "value": [
              {
                "name": "string",
                "number": 0,
                "options": {}
              }
            ]
          }
        ],
        "extension": [
          {
            "defaultValue": "string",
            "extendee": "string",
            "jsonName": "string",
            "label": "Label_VALUE",
            "name": "string",
            "number": 0,
            "oneofIndex": 0,
            "options": {
              "ctype": "CType_VALUE",
              "debugRedact": true,
              "deprecated": true,
              "editionDefaults": [],
              "featureSupport": {},
              "features": {},
              "jstype": "JSType_VALUE",
              "lazy": true,
              "packed": true,
              "retention": "OptionRetention_VALUE",
              "targets": [
                "OptionTargetType_VALUE"
              ],
              "uninterpretedOption": [],
              "unverifiedLazy": true,
              "weak": true
            },
            "proto3Optional": true,
            "type": "Type_VALUE",
            "typeName": "string"
          }
        ],
        "messageType": [
          {
            "enumType": [
              {
                "name": "string",
                "options": {},
                "reservedName": [
                  "string"
                ],
                "reservedRange": [],
                "value": []
              }
            ],
            "extension": [
              {
                "defaultValue": "string",
                "extendee": "string",
                "jsonName": "string",
                "label": "Label_VALUE",
                "name": "string",
                "number": 0,
                "oneofIndex": 0,
                "options": {},
                "proto3Optional": true,
                "type": "Type_VALUE",
                "typeName": "string"
              }
            ],
            "extensionRange": [
              {
                "end": 0,
                "options": {},
                "start": 0
              }
            ],
            "field": [
              {
                "defaultValue": "string",
                "extendee": "string",
                "jsonName": "string",
                "label": "Label_VALUE",
                "name": "string",
                "number": 0,
                "oneofIndex": 0,
                "options": {},
                "proto3Optional": true,
                "type": "Type_VALUE",
                "typeName": "string"
              }
            ],
            "name": "string",
            "nestedType": [
              {
                "enumType": [],
                "extension": [],
                "extensionRange": [],
                "field": [],
                "name": "string",
                "nestedType": [],
                "oneofDecl": [],
                "options": {},
                "reservedName": [
                  "string"
                ],
                "reservedRange": []
              }
            ],
            "oneofDecl": [
              {
                "name": "string",
                "options": {}
              }
            ],
            "options": {
              "deprecated": true,
              "deprecatedLegacyJsonFieldConflicts": true,
              "features": {},
              "mapEntry": true,
              "messageSetWireFormat": true,
              "noStandardDescriptorAccessor": true,
              "uninterpretedOption": []
            },
            "reservedName": [
              "string"
            ],
            "reservedRange": [
              {
                "end": 0,
                "start": 0
              }
            ]
          }
        ],
        "name": "string",
        "options": {
          "ccEnableArenas": true,
          "ccGenericServices": true,
          "csharpNamespace": "string",
          "deprecated": true,
          "features": {
            "enumType": "EnumType_VALUE",
            "fieldPresence": "FieldPresence_VALUE",
            "jsonFormat": "JsonFormat_VALUE",
            "messageEncoding": "MessageEncoding_VALUE",
            "repeatedFieldEncoding": "RepeatedFieldEncoding_VALUE",
            "utf8Validation": "Utf8Validation_VALUE"
          },
          "goPackage": "string",
          "javaGenerateEqualsAndHash": true,
          "javaGenericServices": true,
          "javaMultipleFiles": true,
          "javaOuterClassname": "string",
          "javaPackage": "string",
          "javaStringCheckUtf8": true,
          "objcClassPrefix": "string",
          "optimizeFor": "OptimizeMode_VALUE",
          "phpClassPrefix": "string",
          "phpMetadataNamespace": "string",
          "phpNamespace": "string",
          "pyGenericServices": true,
          "rubyPackage": "string",
          "swiftPrefix": "string",
          "uninterpretedOption": [
            {
              "aggregateValue": "string",
              "doubleValue": 0,
              "identifierValue": "string",
              "name": [],
              "negativeIntValue": 0,
              "positiveIntValue": 0,
              "stringValue": "base64..."
            }
          ]
        },
        "package": "string",
        "publicDependency": [
          0
        ],
        "service": [
          {
            "method": [
              {
                "clientStreaming": true,
                "inputType": "string",
                "name": "string",
                "options": {},
                "outputType": "string",
                "serverStreaming": true
              }
            ],
            "name": "string",
            "options": {
              "deprecated": true,
              "features": {},
              "uninterpretedOption": []
            }
          }
        ],
        "sourceCodeInfo": {
          "location": [
            {
              "leadingComments": "string",
              "leadingDetachedComments": [
                "string"
              ],
              "path": [
                0
              ],
              "span": [
                0
              ],
              "trailingComments": "string"
            }
          ]
        },
        "syntax": "string",
        "weakDependency": [
          0
        ]
      }
    ],
    "sourceFileDescriptors": [
      {
        "dependency": [
          "string"
        ],
        "edition": "Edition_VALUE",
        "enumType": [
          {
            "name": "string",
            "options": {
              "allowAlias": true,
              "deprecated": true,
              "deprecatedLegacyJsonFieldConflicts": true,
              "features": {},
              "uninterpretedOption": []
            },
            "reservedName": [
              "string"
            ],
            "reservedRange": [
              {
                "end": 0,
                "start": 0
              }
            ],
            "value": [
              {
                "name": "string",
                "number": 0,
                "options": {}
              }
            ]
          }
        ],
        "extension": [
          {
            "defaultValue": "string",
            "extendee": "string",
            "jsonName": "string",
            "label": "Label_VALUE",
            "name": "string",
            "number": 0,
            "oneofIndex": 0,
            "options": {
              "ctype": "CType_VALUE",
              "debugRedact": true,
              "deprecated": true,
              "editionDefaults": [],
              "featureSupport": {},
              "features": {},
              "jstype": "JSType_VALUE",
              "lazy": true,
              "packed": true,
              "retention": "OptionRetention_VALUE",
              "targets": [
                "OptionTargetType_VALUE"
              ],
              "uninterpretedOption": [],
              "unverifiedLazy": true,
              "weak": true
            },
            "proto3Optional": true,
            "type": "Type_VALUE",
            "typeName": "string"
          }
        ],
        "messageType": [
          {
            "enumType": [
              {
                "name": "string",
                "options": {},
                "reservedName": [
                  "string"
                ],
                "reservedRange": [],
                "value": []
              }
            ],
            "extension": [
              {
                "defaultValue": "string",
                "extendee": "string",
                "jsonName": "string",
                "label": "Label_VALUE",
                "name": "string",
                "number": 0,
                "oneofIndex": 0,
                "options": {},
                "proto3Optional": true,
                "type": "Type_VALUE",
                "typeName": "string"
              }
            ],
            "extensionRange": [
              {
                "end": 0,
                "options": {},
                "start": 0
              }
            ],
            "field": [
              {
                "defaultValue": "string",
                "extendee": "string",
                "jsonName": "string",
                "label": "Label_VALUE",
                "name": "string",
                "number": 0,
                "oneofIndex": 0,
                "options": {},
                "proto3Optional": true,
                "type": "Type_VALUE",
                "typeName": "string"
              }
            ],
            "name": "string",
            "nestedType": [
              {
                "enumType": [],
                "extension": [],
                "extensionRange": [],
                "field": [],
                "name": "string",
                "nestedType": [],
                "oneofDecl": [],
                "options": {},
                "reservedName": [
                  "string"
                ],
                "reservedRange": []
              }
            ],
            "oneofDecl": [
              {
                "name": "string",
                "options": {}
              }
            ],
            "options": {
              "deprecated": true,
              "deprecatedLegacyJsonFieldConflicts": true,
              "features": {},
              "mapEntry": true,
              "messageSetWireFormat": true,
              "noStandardDescriptorAccessor": true,
              "uninterpretedOption": []
            },
            "reservedName": [
              "string"
            ],
            "reservedRange": [
              {
                "end": 0,
                "start": 0
              }
            ]
          }
        ],
        "name": "string",
        "options": {
          "ccEnableArenas": true,
          "ccGenericServices": true,
          "csharpNamespace": "string",
          "deprecated": true,
          "features": {
            "enumType": "EnumType_VALUE",
            "fieldPresence": "FieldPresence_VALUE",
            "jsonFormat": "JsonFormat_VALUE",
            "messageEncoding": "MessageEncoding_VALUE",
            "repeatedFieldEncoding": "RepeatedFieldEncoding_VALUE",
            "utf8Validation": "Utf8Validation_VALUE"
          },
          "goPackage": "string",
          "javaGenerateEqualsAndHash": true,
          "javaGenericServices": true,
          "javaMultipleFiles": true,
          "javaOuterClassname": "string",
          "javaPackage": "string",
          "javaStringCheckUtf8": true,
          "objcClassPrefix": "string",
          "optimizeFor": "OptimizeMode_VALUE",
          "phpClassPrefix": "string",
          "phpMetadataNamespace": "string",
          "phpNamespace": "string",
          "pyGenericServices": true,
          "rubyPackage": "string",
          "swiftPrefix": "string",
          "uninterpretedOption": [
            {
              "aggregateValue": "string",
              "doubleValue": 0,
              "identifierValue": "string",
              "name": [],
              "negativeIntValue": 0,
              "positiveIntValue": 0,
              "stringValue": "base64..."
            }
          ]
        },
        "package": "string",
        "publicDependency": [
          0
        ],
        "service": [
          {
            "method": [
              {
                "clientStreaming": true,
                "inputType": "string",
                "name": "string",
                "options": {},
                "outputType": "string",
                "serverStreaming": true
              }
            ],
            "name": "string",
            "options": {
              "deprecated": true,
              "features": {},
              "uninterpretedOption": []
            }
          }
        ],
        "sourceCodeInfo": {
          "location": [
            {
              "leadingComments": "string",
              "leadingDetachedComments": [
                "string"
              ],
              "path": [
                0
              ],
              "span": [
                0
              ],
              "trailingComments": "string"
            }
          ]
        },
        "syntax": "string",
        "weakDependency": [
          0
        ]
      }
    ]
  },
  "pluginName": "protocolbuffers/go:v1.36.10"
}
```

#### Response Example

```json
{
  "codeGeneratorResponse": {
    "error": "string",
    "file": [
      {
        "content": "string",
        "generatedCodeInfo": {
          "annotation": [
            {
              "begin": 0,
              "end": 0,
              "path": [
                0
              ],
              "semantic": "Semantic_VALUE",
              "sourceFile": "string"
            }
          ]
        },
        "insertionPoint": "string",
        "name": "string"
      }
    ],
    "maximumEdition": 0,
    "minimumEdition": 0,
    "supportedFeatures": 0
  }
}
```

---

<a name="api-generator-v1-serviceapi-plugins"></a>

### Plugins

```protobuf
rpc Plugins([PluginsRequest](#api-generator-v1-pluginsrequest)) returns ([PluginsResponse](#api-generator-v1-pluginsresponse))
```

List available plugins.

Returns a list of all plugins registered in the service.
Use this to discover available plugins and their versions.

#### Response Example

```json
{
  "plugins": [
    {
      "createdAt": {
        "nanos": 0,
        "seconds": 0
      },
      "group": "protocolbuffers",
      "id": "550e8400-e29b-41d4-a716-446655440000",
      "name": "go",
      "version": "v1.36.10"
    }
  ]
}
```

---

<a name="api-generator-v1-generatecoderequest"></a>

### GenerateCodeRequest

Request message for code generation.

| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| code_generator_request | [CodeGeneratorRequest](#google-protobuf-compiler-codegeneratorrequest) | optional | **Required** Standard protobuf code generator request.  This should contain the proto files to process and any plugin-specific parameters. The request is passed directly to the plugin's stdin. |
| plugin_name | string | optional | **Required** Name of the plugin to use for generation.  Format: `<group>/<name>:<version>`  Examples: - `protocolbuffers/go:v1.36.10` - `grpc/go:v1.5.1` - `grpc-ecosystem/gateway:latest` *pattern: `^[a-z][a-z0-9-]*/[a-z][a-z0-9-]*:(v[0-9]+\.[0-9]+\.[0-9]+|latest)$`* Example: `protocolbuffers/go:v1.36.10` |

<details>
<summary>JSON Example</summary>

```json
{
  "codeGeneratorRequest": {
    "compilerVersion": {
      "major": 0,
      "minor": 0,
      "patch": 0,
      "suffix": "string"
    },
    "fileToGenerate": [
      "string"
    ],
    "parameter": "string",
    "protoFile": [
      {
        "dependency": [
          "string"
        ],
        "edition": "Edition_VALUE",
        "enumType": [
          {
            "name": "string",
            "options": {
              "allowAlias": true,
              "deprecated": true,
              "deprecatedLegacyJsonFieldConflicts": true,
              "features": {},
              "uninterpretedOption": []
            },
            "reservedName": [
              "string"
            ],
            "reservedRange": [
              {
                "end": 0,
                "start": 0
              }
            ],
            "value": [
              {
                "name": "string",
                "number": 0,
                "options": {}
              }
            ]
          }
        ],
        "extension": [
          {
            "defaultValue": "string",
            "extendee": "string",
            "jsonName": "string",
            "label": "Label_VALUE",
            "name": "string",
            "number": 0,
            "oneofIndex": 0,
            "options": {
              "ctype": "CType_VALUE",
              "debugRedact": true,
              "deprecated": true,
              "editionDefaults": [],
              "featureSupport": {},
              "features": {},
              "jstype": "JSType_VALUE",
              "lazy": true,
              "packed": true,
              "retention": "OptionRetention_VALUE",
              "targets": [
                "OptionTargetType_VALUE"
              ],
              "uninterpretedOption": [],
              "unverifiedLazy": true,
              "weak": true
            },
            "proto3Optional": true,
            "type": "Type_VALUE",
            "typeName": "string"
          }
        ],
        "messageType": [
          {
            "enumType": [
              {
                "name": "string",
                "options": {},
                "reservedName": [
                  "string"
                ],
                "reservedRange": [],
                "value": []
              }
            ],
            "extension": [
              {
                "defaultValue": "string",
                "extendee": "string",
                "jsonName": "string",
                "label": "Label_VALUE",
                "name": "string",
                "number": 0,
                "oneofIndex": 0,
                "options": {},
                "proto3Optional": true,
                "type": "Type_VALUE",
                "typeName": "string"
              }
            ],
            "extensionRange": [
              {
                "end": 0,
                "options": {},
                "start": 0
              }
            ],
            "field": [
              {
                "defaultValue": "string",
                "extendee": "string",
                "jsonName": "string",
                "label": "Label_VALUE",
                "name": "string",
                "number": 0,
                "oneofIndex": 0,
                "options": {},
                "proto3Optional": true,
                "type": "Type_VALUE",
                "typeName": "string"
              }
            ],
            "name": "string",
            "nestedType": [
              {
                "enumType": [],
                "extension": [],
                "extensionRange": [],
                "field": [],
                "name": "string",
                "nestedType": [],
                "oneofDecl": [],
                "options": {},
                "reservedName": [
                  "string"
                ],
                "reservedRange": []
              }
            ],
            "oneofDecl": [
              {
                "name": "string",
                "options": {}
              }
            ],
            "options": {
              "deprecated": true,
              "deprecatedLegacyJsonFieldConflicts": true,
              "features": {},
              "mapEntry": true,
              "messageSetWireFormat": true,
              "noStandardDescriptorAccessor": true,
              "uninterpretedOption": []
            },
            "reservedName": [
              "string"
            ],
            "reservedRange": [
              {
                "end": 0,
                "start": 0
              }
            ]
          }
        ],
        "name": "string",
        "options": {
          "ccEnableArenas": true,
          "ccGenericServices": true,
          "csharpNamespace": "string",
          "deprecated": true,
          "features": {
            "enumType": "EnumType_VALUE",
            "fieldPresence": "FieldPresence_VALUE",
            "jsonFormat": "JsonFormat_VALUE",
            "messageEncoding": "MessageEncoding_VALUE",
            "repeatedFieldEncoding": "RepeatedFieldEncoding_VALUE",
            "utf8Validation": "Utf8Validation_VALUE"
          },
          "goPackage": "string",
          "javaGenerateEqualsAndHash": true,
          "javaGenericServices": true,
          "javaMultipleFiles": true,
          "javaOuterClassname": "string",
          "javaPackage": "string",
          "javaStringCheckUtf8": true,
          "objcClassPrefix": "string",
          "optimizeFor": "OptimizeMode_VALUE",
          "phpClassPrefix": "string",
          "phpMetadataNamespace": "string",
          "phpNamespace": "string",
          "pyGenericServices": true,
          "rubyPackage": "string",
          "swiftPrefix": "string",
          "uninterpretedOption": [
            {
              "aggregateValue": "string",
              "doubleValue": 0,
              "identifierValue": "string",
              "name": [],
              "negativeIntValue": 0,
              "positiveIntValue": 0,
              "stringValue": "base64..."
            }
          ]
        },
        "package": "string",
        "publicDependency": [
          0
        ],
        "service": [
          {
            "method": [
              {
                "clientStreaming": true,
                "inputType": "string",
                "name": "string",
                "options": {},
                "outputType": "string",
                "serverStreaming": true
              }
            ],
            "name": "string",
            "options": {
              "deprecated": true,
              "features": {},
              "uninterpretedOption": []
            }
          }
        ],
        "sourceCodeInfo": {
          "location": [
            {
              "leadingComments": "string",
              "leadingDetachedComments": [
                "string"
              ],
              "path": [
                0
              ],
              "span": [
                0
              ],
              "trailingComments": "string"
            }
          ]
        },
        "syntax": "string",
        "weakDependency": [
          0
        ]
      }
    ],
    "sourceFileDescriptors": [
      {
        "dependency": [
          "string"
        ],
        "edition": "Edition_VALUE",
        "enumType": [
          {
            "name": "string",
            "options": {
              "allowAlias": true,
              "deprecated": true,
              "deprecatedLegacyJsonFieldConflicts": true,
              "features": {},
              "uninterpretedOption": []
            },
            "reservedName": [
              "string"
            ],
            "reservedRange": [
              {
                "end": 0,
                "start": 0
              }
            ],
            "value": [
              {
                "name": "string",
                "number": 0,
                "options": {}
              }
            ]
          }
        ],
        "extension": [
          {
            "defaultValue": "string",
            "extendee": "string",
            "jsonName": "string",
            "label": "Label_VALUE",
            "name": "string",
            "number": 0,
            "oneofIndex": 0,
            "options": {
              "ctype": "CType_VALUE",
              "debugRedact": true,
              "deprecated": true,
              "editionDefaults": [],
              "featureSupport": {},
              "features": {},
              "jstype": "JSType_VALUE",
              "lazy": true,
              "packed": true,
              "retention": "OptionRetention_VALUE",
              "targets": [
                "OptionTargetType_VALUE"
              ],
              "uninterpretedOption": [],
              "unverifiedLazy": true,
              "weak": true
            },
            "proto3Optional": true,
            "type": "Type_VALUE",
            "typeName": "string"
          }
        ],
        "messageType": [
          {
            "enumType": [
              {
                "name": "string",
                "options": {},
                "reservedName": [
                  "string"
                ],
                "reservedRange": [],
                "value": []
              }
            ],
            "extension": [
              {
                "defaultValue": "string",
                "extendee": "string",
                "jsonName": "string",
                "label": "Label_VALUE",
                "name": "string",
                "number": 0,
                "oneofIndex": 0,
                "options": {},
                "proto3Optional": true,
                "type": "Type_VALUE",
                "typeName": "string"
              }
            ],
            "extensionRange": [
              {
                "end": 0,
                "options": {},
                "start": 0
              }
            ],
            "field": [
              {
                "defaultValue": "string",
                "extendee": "string",
                "jsonName": "string",
                "label": "Label_VALUE",
                "name": "string",
                "number": 0,
                "oneofIndex": 0,
                "options": {},
                "proto3Optional": true,
                "type": "Type_VALUE",
                "typeName": "string"
              }
            ],
            "name": "string",
            "nestedType": [
              {
                "enumType": [],
                "extension": [],
                "extensionRange": [],
                "field": [],
                "name": "string",
                "nestedType": [],
                "oneofDecl": [],
                "options": {},
                "reservedName": [
                  "string"
                ],
                "reservedRange": []
              }
            ],
            "oneofDecl": [
              {
                "name": "string",
                "options": {}
              }
            ],
            "options": {
              "deprecated": true,
              "deprecatedLegacyJsonFieldConflicts": true,
              "features": {},
              "mapEntry": true,
              "messageSetWireFormat": true,
              "noStandardDescriptorAccessor": true,
              "uninterpretedOption": []
            },
            "reservedName": [
              "string"
            ],
            "reservedRange": [
              {
                "end": 0,
                "start": 0
              }
            ]
          }
        ],
        "name": "string",
        "options": {
          "ccEnableArenas": true,
          "ccGenericServices": true,
          "csharpNamespace": "string",
          "deprecated": true,
          "features": {
            "enumType": "EnumType_VALUE",
            "fieldPresence": "FieldPresence_VALUE",
            "jsonFormat": "JsonFormat_VALUE",
            "messageEncoding": "MessageEncoding_VALUE",
            "repeatedFieldEncoding": "RepeatedFieldEncoding_VALUE",
            "utf8Validation": "Utf8Validation_VALUE"
          },
          "goPackage": "string",
          "javaGenerateEqualsAndHash": true,
          "javaGenericServices": true,
          "javaMultipleFiles": true,
          "javaOuterClassname": "string",
          "javaPackage": "string",
          "javaStringCheckUtf8": true,
          "objcClassPrefix": "string",
          "optimizeFor": "OptimizeMode_VALUE",
          "phpClassPrefix": "string",
          "phpMetadataNamespace": "string",
          "phpNamespace": "string",
          "pyGenericServices": true,
          "rubyPackage": "string",
          "swiftPrefix": "string",
          "uninterpretedOption": [
            {
              "aggregateValue": "string",
              "doubleValue": 0,
              "identifierValue": "string",
              "name": [],
              "negativeIntValue": 0,
              "positiveIntValue": 0,
              "stringValue": "base64..."
            }
          ]
        },
        "package": "string",
        "publicDependency": [
          0
        ],
        "service": [
          {
            "method": [
              {
                "clientStreaming": true,
                "inputType": "string",
                "name": "string",
                "options": {},
                "outputType": "string",
                "serverStreaming": true
              }
            ],
            "name": "string",
            "options": {
              "deprecated": true,
              "features": {},
              "uninterpretedOption": []
            }
          }
        ],
        "sourceCodeInfo": {
          "location": [
            {
              "leadingComments": "string",
              "leadingDetachedComments": [
                "string"
              ],
              "path": [
                0
              ],
              "span": [
                0
              ],
              "trailingComments": "string"
            }
          ]
        },
        "syntax": "string",
        "weakDependency": [
          0
        ]
      }
    ]
  },
  "pluginName": "protocolbuffers/go:v1.36.10"
}
```

</details>

<a name="api-generator-v1-generatecoderesponse"></a>

### GenerateCodeResponse

Response message for code generation.

| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| code_generator_response | [CodeGeneratorResponse](#google-protobuf-compiler-codegeneratorresponse) | optional | `Output Only` Standard protobuf code generator response.  Contains the generated files and any error messages from the plugin. Check the `error` field in the response for plugin-level errors. |

<details>
<summary>JSON Example</summary>

```json
{
  "codeGeneratorResponse": {
    "error": "string",
    "file": [
      {
        "content": "string",
        "generatedCodeInfo": {
          "annotation": [
            {
              "begin": 0,
              "end": 0,
              "path": [
                0
              ],
              "semantic": "Semantic_VALUE",
              "sourceFile": "string"
            }
          ]
        },
        "insertionPoint": "string",
        "name": "string"
      }
    ],
    "maximumEdition": 0,
    "minimumEdition": 0,
    "supportedFeatures": 0
  }
}
```

</details>

<a name="api-generator-v1-pluginsrequest"></a>

### PluginsRequest

Request message for listing plugins.

Currently accepts no parameters. Future versions may add filtering options.

<a name="api-generator-v1-pluginsresponse"></a>

### PluginsResponse

Response message for listing plugins.

| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| plugins | [PluginInfo](#api-generator-v1-plugininfo) | repeated | `Output Only` List of available plugins.  Plugins are sorted by group, name, and version. |

<details>
<summary>JSON Example</summary>

```json
{
  "plugins": [
    {
      "createdAt": {
        "nanos": 0,
        "seconds": 0
      },
      "group": "protocolbuffers",
      "id": "550e8400-e29b-41d4-a716-446655440000",
      "name": "go",
      "version": "v1.36.10"
    }
  ]
}
```

</details>

<a name="api-generator-v1-plugininfo"></a>

### PluginInfo

Information about a registered plugin.

| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| id | string | optional | `Output Only` `uuid` Unique identifier for the plugin.  This is an internal UUID assigned when the plugin is registered. |
| group | string | optional | `Output Only` Group to which the plugin belongs.  Groups organize plugins by maintainer or ecosystem.  Common groups: - `protocolbuffers` — Official Google protobuf plugins - `grpc` — Official gRPC plugins - `grpc-ecosystem` — gRPC ecosystem plugins (gateway, openapi) - `community` — Community-maintained plugins *pattern: `^[a-z][a-z0-9-]*$`* Example: `protocolbuffers` |
| name | string | optional | `Output Only` Name of the plugin.  This is the plugin's identifier within its group.  Examples: `go`, `python`, `gateway`, `openapiv2` *pattern: `^[a-z][a-z0-9-]*$`* Example: `go` |
| version | string | optional | `Output Only` Version of the plugin.  Follows semantic versioning (semver) format. *pattern: `^v[0-9]+\.[0-9]+\.[0-9]+$`* Example: `v1.36.10` |
| created_at | [Timestamp](#google-protobuf-timestamp) | optional | `Output Only` Timestamp when the plugin was registered. |

<details>
<summary>JSON Example</summary>

```json
{
  "createdAt": {
    "nanos": 0,
    "seconds": 0
  },
  "group": "protocolbuffers",
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "name": "go",
  "version": "v1.36.10"
}
```

</details>


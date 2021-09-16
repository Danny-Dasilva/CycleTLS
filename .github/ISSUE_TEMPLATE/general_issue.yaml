name: CycleTLS Issue Template
description: Use this template to report any issue
labels: [bug, triage]
assignees:
  - Danny-Dasilva
body:
  - type: markdown
    attributes:
      value: |
        Thank you for taking the time to report a CycleTLS issue
  - type: textarea
    id: what-happened
    attributes:
      label: Description
      description: Please describe the current and expected behaviour, and attach all files/info needed to reproduce the issue if applicable.
#       placeholder: Describe the issue here!
#       value: "A bug happened!"
    validations:
      required: true
  - type: dropdown
    id: issue-type
    attributes:
      label: Issue Type
      description: What type of issue would you like to report?
      multiple: true
      options:
        - Bug
        - Build/Install
        - Performance
        - Support
        - Feature Request
        - Documentation Feature Request
        - Documentation Bug
        - Others
  - type: dropdown
    id: Operating-System
    attributes:
      label: Operating System
      description: What OS are you seeing the issue in? If you don't see your OS listed, please provide more details in the "Description" section above.
      multiple: true
      options:
        - Windows 10
        - Linux
        - Ubuntu
        - Mac OS
        - Other
  - type: dropdown
    id: version
    attributes:
      label: Node Version
      description: What Node version are you using? If "other", please provide more details in the "Description" section above.
      multiple: false
      options:
        - C++
        - Node 14.x
        - Node 15.x
        - Node 16.x
        - Other
  - type: dropdown
    id: version
    attributes:
      label: Golang Version
      description: What Golang version are you using? If "other", please provide more details in the "Description" section above.
      multiple: false
      options:
        - C++
        - Go 14.x
        - Go 15.x
        - Go 16.x
        - Go 17.x
        - Other
  - type: textarea
    id: logs
    attributes:
      label: Relevant Log Output
      description: Please copy and paste any relevant log output. This will be automatically formatted into code.
      render: shell

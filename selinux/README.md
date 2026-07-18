## SELinux policy for webhook

### Build & Install

Install the selinux-policy-devel package, then run:

    cd webhook/selinux
    make -f /usr/share/selinux/devel/Makefile

It should produce the `webhook.pp` file.

Install the module:

    sudo semodule -i webhook.pp
    sudo restorecon /usr/bin/webhook /etc/webhook.conf

Label any webhook scripts as `webhook_unconfined_exec_t` with `chcon`:

    sudo chcon -t webhook_unconfined_exec_t /usr/local/bin/deploy.sh

### Policy

The policy defines the following domain types

* `webhook_t`
* `webhook_exec_t`
* `webhook_unconfined_t`
* `webhook_unconfined_exec_t`

When `init_t` launches the `webhook` program, it will automatically
transition to the `webhook_t` domain.

`webhook_t` is able to run any executables or scripts labeled with
the `webhook_unconfined_exec_t` file type. Those will run in the
unconfined `webhook_unconfined_t` domain. Other executables will
run with the `webhook_t` domain with minimum permissions.

The policy also defines a default `on` boolean, `webhook_unconfined_exec_any`,
which allow running programs labeled as `bin_t` (files in `/usr/bin` or `/usr/local/bin`)
in the `webhook_unconfined_t` domain.

Set the boolean to `off` to enable maximum protection:

    sudo setsebool webhook_unconfined_exec_any off

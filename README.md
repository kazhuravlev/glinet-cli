# GL.Inet router console client

This client is allows to automate operations
with [Gl.Inet](https://www.gl-inet.com/) devices
through [http API](https://dev.gl-inet.com/api/).

## Installation

**By go install**

```shell
go install github.com/kazhuravlev/glinet-cli/cmd/glinet@latest
```

**By Homebrew**

```shell
brew install kazhuravlev/glinet/glinet
```

## Preparations

This tool will work to your router only when:

- Your machine connected to the router
- You know the router IP (usually - `192.168.8.1`)
- You know the router password, that using for login into admin interface

To make sure, that all is good - try to login into your router admin interface
using browser.
[Troubleshooting](https://docs.gl-inet.com/en/3/tutorials/cannot_access_web_admin_panel/)

## Notes

This tool use tls for each request. But, you should understand that it is not
possible to make sure the validity of certificate.

## Authorization

> Note
>
> The Gl.Inet API uses the old authentication method. To do this, you need to
> send your password to the API endpoint and receive an API token in response.
> Sometimes token is expired (not predictable in all cases), so this tool should
> store your password at config file.

After you log in with this tool, you will receive a token which will be stored
in the `~/.config/glinet` directory. Password also will store in a config to
using for auto-renewal session.

```shell
# This command will ask you about password
glinet auth 192.168.8.1 
```

## Features

[//]: # (TODO: describe)

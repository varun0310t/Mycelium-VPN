#
# This script enables Internet Connection Sharing (ICS).
# It takes the name of the public (internet) adapter and the private (TUN) adapter.
#
param (
    [string]$publicAdapterName,
    [string]$privateAdapterName
)

function Set-ICS {
    param (
        [string]$publicName,
        [string]$privateName
    )

    Write-Output "Setting ICS to share '$publicName' with '$privateName'..."

    # Create a NetSharingManager object
    $netShare = New-Object -ComObject HNetCfg.HNetShare

    # Find the public (internet) connection
    $publicConnection = $netShare.EnumEveryConnection | Where-Object {
        $netShare.NetConnectionProps.Invoke($_).Name -eq $publicName
    }

    # Find the private (TUN) connection
    $privateConnection = $netShare.EnumEveryConnection | Where-Object {
        $netShare.NetConnectionProps.Invoke($_).Name -eq $privateName
    }

    if (-not $publicConnection -or -not $privateConnection) {
        Write-Error "Error: One or both network adapters not found."
        return
    }

    # Get the sharing configuration for both connections
    $publicConfig = $netShare.INetSharingConfigurationForINetConnection.Invoke($publicConnection)
    $privateConfig = $netShare.INetSharingConfigurationForINetConnection.Invoke($privateConnection)

    # Disable previous sharing to reset everything
    $publicConfig.DisableSharing()
    $privateConfig.DisableSharing()

    # Enable sharing on the public connection for the private network
    $publicConfig.EnableSharing(0) # 0 = Public
    $privateConfig.EnableSharing(1) # 1 = Private

    Write-Output "✅ ICS configured successfully."
}

# Run the function with the provided adapter names
Set-ICS -publicName $publicAdapterName -privateName $privateAdapterName
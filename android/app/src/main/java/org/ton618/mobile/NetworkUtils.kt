package org.ton618.mobile

import android.content.Context
import android.net.ConnectivityManager
import android.net.NetworkCapabilities
import android.net.wifi.WifiManager
import java.net.Inet4Address
import java.net.InetAddress
import java.net.NetworkInterface
import java.net.Socket

object NetworkUtils {

    /**
     * Retorna o endereço IP IPv4 local do dispositivo na rede Wi-Fi / Ethernet.
     */
    fun getLocalIpAddress(context: Context): String {
        try {
            val interfaces = NetworkInterface.getNetworkInterfaces()
            while (interfaces.hasMoreElements()) {
                val intf = interfaces.nextElement()
                if (intf.isLoopback || !intf.isUp) continue
                val addresses = intf.inetAddresses
                while (addresses.hasMoreElements()) {
                    val addr = addresses.nextElement()
                    if (!addr.isLoopbackAddress && addr is Inet4Address) {
                        return addr.hostAddress ?: "127.0.0.1"
                    }
                }
            }
        } catch (e: Exception) {
            e.printStackTrace()
        }
        return "127.0.0.1"
    }

    /**
     * Testa se a porta TCP especificada está respondendo em localhost.
     */
    fun isServerPortOpen(port: Int, timeoutMs: Int = 300): Boolean {
        return try {
            Socket("127.0.0.1", port).use { true }
        } catch (e: Exception) {
            false
        }
    }
}

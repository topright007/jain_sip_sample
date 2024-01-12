package com.topright007.voicemenusip

import jakarta.annotation.PostConstruct
import jakarta.annotation.PreDestroy
import net.java.sip.communicator.impl.certificate.CertificateVerificationActivator
import net.java.sip.communicator.impl.configuration.ConfigurationActivator
import net.java.sip.communicator.impl.credentialsstorage.CredentialsStorageActivator
import net.java.sip.communicator.impl.dns.DnsUtilActivator
import net.java.sip.communicator.impl.globaldisplaydetails.GlobalDisplayDetailsActivator
import net.java.sip.communicator.impl.neomedia.NeomediaActivator
import net.java.sip.communicator.impl.netaddr.NetaddrActivator
import net.java.sip.communicator.impl.packetlogging.PacketLoggingActivator
import net.java.sip.communicator.impl.protocol.sip.SipActivator
import net.java.sip.communicator.impl.resources.ResourceManagementActivator
import net.java.sip.communicator.service.gui.internal.GuiServiceActivator
import net.java.sip.communicator.service.notification.NotificationServiceActivator
import net.java.sip.communicator.service.protocol.OperationSetBasicTelephony
import net.java.sip.communicator.service.protocol.ProtocolProviderActivator
import net.java.sip.communicator.service.protocol.media.ProtocolMediaActivator
import net.java.sip.communicator.util.UtilActivator
import org.jitsi.impl.osgi.framework.launch.FrameworkImpl
import org.jitsi.osgi.framework.BundleActivatorHolder
import org.jitsi.service.configuration.ConfigurationService
import org.jitsi.service.libjitsi.LibJitsiActivator
import org.jitsi.service.neomedia.DefaultStreamConnector
import org.osgi.framework.BundleActivator
import org.osgi.framework.BundleException
import org.osgi.framework.Constants
import org.osgi.framework.launch.Framework
import org.osgi.framework.startlevel.BundleStartLevel
import org.springframework.stereotype.Service

@Service
class SipBootstrap {
    var fw: Framework? = null

    @PostConstruct
    public fun postConstruct() {

        // Jingle Raw UDP transport
        System.setProperty(DefaultStreamConnector.MAX_PORT_NUMBER_PROPERTY_NAME, "20000")
        // Jingle ICE-UDP transport
        // Jingle ICE-UDP transport
        System.setProperty(OperationSetBasicTelephony.MAX_MEDIA_PORT_NUMBER_PROPERTY_NAME, "20000")

        // Jingle Raw UDP transport
        System.setProperty(DefaultStreamConnector.MIN_PORT_NUMBER_PROPERTY_NAME, "10000")
        // Jingle ICE-UDP transport
        // Jingle ICE-UDP transport
        System.setProperty(OperationSetBasicTelephony.MIN_MEDIA_PORT_NUMBER_PROPERTY_NAME, "10000")

        // FIXME: properties used for debug purposes
        // jigasi-home will be create in current directory (from where the
        // process is launched). It must contain sip-communicator.properties
        // with one XMPP and one SIP account configured.
        val configDir = "./config"
        val homeDir = "./home"

        System.setProperty(ConfigurationService.PNAME_SC_HOME_DIR_LOCATION, configDir)

        System.setProperty(ConfigurationService.PNAME_SC_HOME_DIR_NAME, homeDir)

        System.setProperty(ConfigurationService.PNAME_CONFIGURATION_FILE_IS_READ_ONLY, "true")

        // make sure we use the properties files for configuration
        System.setProperty(ConfigurationActivator.PNAME_USE_PROPFILE_CONFIG, "true")

        // Those are not used so disable them
        val deviceSystemPackage = "org.jitsi.impl.neomedia.device"
        System.setProperty("$deviceSystemPackage.MacCoreaudioSystem.disabled", "true")
        System.setProperty("$deviceSystemPackage.PulseAudioSystem.disabled", "true")
        System.setProperty("$deviceSystemPackage.PortAudioSystem.disabled", "true")

        val protocols: List<Class<out BundleActivator?>> = java.util.List.of(
            SipActivator::class.java,
        )
        fw = start(protocols)

        // register shutdown service so we can cleanly stop the framework
    }

    @PreDestroy
    public fun preDestroy() {
        try {
            fw?.stop()
            // give some time to tear down everything before exiting
            Thread.sleep(3000)
        } catch (e: Exception) {
            e.printStackTrace()
        }
    }

    @Throws(BundleException::class, InterruptedException::class)
    fun start(protocols: List<Class<out BundleActivator?>>?): Framework? {
        val activators: MutableList<Class<out BundleActivator?>> = mutableListOf(
            LibJitsiActivator::class.java,
            ConfigurationActivator::class.java,
            UtilActivator::class.java,
            ResourceManagementActivator::class.java,
            NotificationServiceActivator::class.java,
            DnsUtilActivator::class.java,
            CredentialsStorageActivator::class.java,
            NetaddrActivator::class.java,
            PacketLoggingActivator::class.java,
            GuiServiceActivator::class.java,
            ProtocolMediaActivator::class.java,
            NeomediaActivator::class.java,
            CertificateVerificationActivator::class.java,
            ProtocolProviderActivator::class.java,
            GlobalDisplayDetailsActivator::class.java
        )
        activators.addAll(protocols!!)

        val options = HashMap<String, String>()
        options[Constants.FRAMEWORK_BEGINNING_STARTLEVEL] = "3"
        val fw: Framework = FrameworkImpl(options, this.javaClass.classLoader)
        fw.init()
        val bundleContext = fw.bundleContext
        for (activator in activators) {
            val url = activator.protectionDomain.codeSource.location.toString()
            val bundle = bundleContext.installBundle(url)
            val startLevel = bundle.adapt(BundleStartLevel::class.java)
            startLevel.startLevel = 2
            val bundleActivator = bundle.adapt(BundleActivatorHolder::class.java)
            bundleActivator.addBundleActivator(activator)
        }
        fw.start()
        return fw
    }
}
-- Add Linphone softphone provisioning profile

INSERT INTO provisioning_profiles (name, vendor, model, description, config_template, variables, is_default) VALUES (
    'Linphone Default',
    'linphone',
    NULL,
    'Remote provisioning template for Linphone softphone (iOS, Android, Desktop). Supports XML format per Linphone remote provisioning specification.',
    '<?xml version="1.0" encoding="UTF-8"?>
<config xmlns="http://www.linphone.org/xsds/lpconfig.xsd"
        xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
        xsi:schemaLocation="http://www.linphone.org/xsds/lpconfig.xsd lpconfig.xsd">

    <!-- Apply settings only once, not on each app restart -->
    <section name="misc">
        <entry name="transient_provisioning" overwrite="true">1</entry>
    </section>

    <!-- SIP Transport Settings -->
    <section name="sip">
        <entry name="sip_port" overwrite="true">-1</entry>
        <entry name="sip_tcp_port" overwrite="true">-1</entry>
        <entry name="sip_tls_port" overwrite="true">-1</entry>
        <entry name="default_proxy" overwrite="true">0</entry>
        <entry name="register_only_when_network_is_up" overwrite="true">1</entry>
    </section>

    <!-- Proxy/Account Configuration -->
    <section name="proxy_0">
        <entry name="reg_proxy" overwrite="true">sip:{{.SIPServer}}:{{.SIPPort}}</entry>
        <entry name="reg_identity" overwrite="true">sip:{{.Username}}@{{.SIPServer}}</entry>
        <entry name="reg_expires" overwrite="true">600</entry>
        <entry name="reg_sendregister" overwrite="true">1</entry>
        <entry name="publish" overwrite="true">0</entry>
        <entry name="dial_escape_plus" overwrite="true">0</entry>
        <entry name="quality_reporting_enabled" overwrite="true">0</entry>
        <entry name="avpf" overwrite="true">-1</entry>
        <entry name="avpf_rr_interval" overwrite="true">1</entry>
        <entry name="nat_policy_ref" overwrite="true">nat_policy_0</entry>
    </section>

    <!-- Authentication Credentials -->
    <section name="auth_info_0">
        <entry name="username" overwrite="true">{{.Username}}</entry>
        <entry name="userid" overwrite="true">{{.AuthID}}</entry>
        <entry name="passwd" overwrite="true">{{.AuthPassword}}</entry>
        <entry name="realm" overwrite="true">{{.SIPServer}}</entry>
        <entry name="domain" overwrite="true">{{.SIPServer}}</entry>
        <entry name="algorithm" overwrite="true">MD5</entry>
    </section>

    <!-- NAT Traversal Policy -->
    <section name="nat_policy_0">
        <entry name="stun_server" overwrite="true">{{.STUNServer}}</entry>
        <entry name="protocols" overwrite="true">stun,ice</entry>
        <entry name="stun_server_username" overwrite="true"></entry>
    </section>

    <!-- RTP Settings -->
    <section name="rtp">
        <entry name="audio_rtp_port" overwrite="true">7078</entry>
        <entry name="video_rtp_port" overwrite="true">9078</entry>
        <entry name="audio_jitt_comp" overwrite="true">60</entry>
        <entry name="video_jitt_comp" overwrite="true">60</entry>
        <entry name="nortp_timeout" overwrite="true">30</entry>
    </section>

    <!-- Audio Codec Preferences (G.711u, G.711a, Opus) -->
    <section name="audio_codec_0">
        <entry name="mime" overwrite="true">PCMU</entry>
        <entry name="rate" overwrite="true">8000</entry>
        <entry name="channels" overwrite="true">1</entry>
        <entry name="enabled" overwrite="true">1</entry>
    </section>
    <section name="audio_codec_1">
        <entry name="mime" overwrite="true">PCMA</entry>
        <entry name="rate" overwrite="true">8000</entry>
        <entry name="channels" overwrite="true">1</entry>
        <entry name="enabled" overwrite="true">1</entry>
    </section>
    <section name="audio_codec_2">
        <entry name="mime" overwrite="true">opus</entry>
        <entry name="rate" overwrite="true">48000</entry>
        <entry name="channels" overwrite="true">2</entry>
        <entry name="enabled" overwrite="true">1</entry>
    </section>

    <!-- Video disabled by default for bandwidth -->
    <section name="video">
        <entry name="display" overwrite="true">0</entry>
        <entry name="capture" overwrite="true">0</entry>
        <entry name="show_local" overwrite="true">0</entry>
        <entry name="automatically_initiate" overwrite="true">0</entry>
        <entry name="automatically_accept" overwrite="true">0</entry>
    </section>

    <!-- MWI (Message Waiting Indicator) -->
    <section name="sip">
        <entry name="subscribe_expires" overwrite="true">600</entry>
    </section>

</config>',
    '{"SIPServer": "", "SIPPort": "5060", "AuthID": "", "AuthPassword": "", "DisplayName": "", "Username": "", "STUNServer": "stun.l.google.com:19302"}',
    1
);

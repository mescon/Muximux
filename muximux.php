<?php
/*
* DO NOT CHANGE THIS FILE!
* The settings are all in config.ini.php
*/

try {
    $config = parse_ini_file('config.ini.php', true);
} catch(Exception $e) {
    die('<b>Unable to read config.ini.php. Did you rename it from config.ini.php-example?</b><br><br>Error message: ' .$e->getMessage());
}


function menuItems($config) {
    if (empty($item)) $item = '';
    foreach ($config as $keyname => $section) {
        if(!empty($section["enabled"]) && !($section["enabled"]=="false") && ($section["enabled"]=="true")) {
            if(!empty($section["default"]) && !($section["default"]=="false") && ($section["default"]=="true")) {
                $item .= "<li><a data-content=\"" . $keyname . "\" class=\"selected\"><span class=\"". $section["icon"] ." fa-lg\"></span> ". $section["name"] ."</a></li>\n";
            } else {
                    $item .= "<li><a data-content=\"" . $keyname . "\"><span class=\"". $section["icon"] ." fa-lg\"></span> ". $section["name"] ."</a></li>\n";
            }
        }
    }
    return $item;
}

function getTitle($config){
    if (empty($item)) $item = 'Muximux - Application Management Console';
    foreach ($config as $keyname => $section) {
        if(($keyname == "general")) {
            $item = $section["title"];
            break;
        }
    }
    return $item;
}


function frameContent($config) {
    if (empty($item)) $item = '';
    foreach ($config as $keyname => $section) {
        if(!empty($section["landingpage"]) && !($section["landingpage"]=="false") && ($section["landingpage"]=="true")) {
            $section["url"] = "?landing=" . $keyname;
        }

        if(!empty($section["enabled"]) && !($section["enabled"]=="false") && ($section["enabled"]=="true")) {
            if(!empty($section["default"]) && !($section["default"]=="false") && ($section["default"]=="true")) {
                $item .= "<li data-content=\"". $keyname . "\" class=\"selected\">";
            } else {
                $item .= "<li data-content=\"". $keyname . "\">";
            }

            $item .= "<iframe allowfullscreen=\"true\" webkitallowfullscreen=\"true\" mozallowfullscreen=\"true\" scrolling=\"auto\" data-src=\"". $section["url"] . "\"></iframe></li>\n";
        }
    }
    return $item;
}


function landingPage($config, $keyname) {
    $item = "
    <html lang=\"en\">
    <head>
    <title>". $config[$keyname]["name"] ."</title>
    <link rel=\"stylesheet\" href=\"css/landing.css\">
    </head>
    <body>
    <div class=\"login\">
        <div class=\"heading\">
            <h2><span class=\"". $config[$keyname]["icon"] ." fa-3x\"></span></h2>
            <section>
                <a href=\"". $config[$keyname]["url"] ."\" target=\"_self\" title=\"Launch ". $config[$keyname]["name"] ."!\"><button class=\"float\">Launch ". $config[$keyname]["name"] ."</button></a>
            </section>
        </div>
     </div>
     </body></html>";
    if (empty($item)) $item = '';
    return $item;
}

if(isset($_GET['landing'])) {
    $keyname = $_GET['landing'];
    echo landingPage($config, $keyname);
    die();
}
?>

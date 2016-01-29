<?php
/*
* DO NOT CHANGE THIS FILE!
*/
define('CONFIG', 'settings.ini.php');
define('CONFIGEXAMPLE', 'settings.ini.php-example');
require __DIR__ . '/vendor/autoload.php';

// Check if this is an old installation that needs upgrading.
if (file_exists('config.ini.php')) {
    copy('config.ini.php', 'backup.ini.php');
    unlink('config.ini.php');
    $upgrade = true;
} else {
    $upgrade = false;
}

if(!file_exists(CONFIG)){
    if (!is_writable(dirname('settings.ini.php-example')))
        die('The directory Muximux is installed in does not have write permissions. Please make sure your apache/nginx/IIS/lightHttpd user has write permissions to this folder');
    else {
        copy(CONFIGEXAMPLE, CONFIG);
    }
}

// First what we're gonna do - save or read
if (sizeof($_POST) > 0) {
    write_ini();
} else {
    parse_ini();
}



function write_ini()
{
    unlink(CONFIG);

    $config = new Config_Lite(CONFIG);
    foreach ($_POST as $parameter => $value) {
        $splitParameter = explode('-', $parameter);
        if ($value == "on")
            $value = "true";
        $config->set($splitParameter[0], $splitParameter[1], $value);
    }
    // save object to file
    try {
        $config->save();
    } catch (Config_Lite_Exception $e) {
        echo "\n", 'Exception Message: ', $e->getMessage();
    } finally {
        echo true;
    }

    $cache_new = "; <?php ;die(\"Access denied\"); ?>"; // Adds this to the top of the config so that PHP kills the execution if someone tries to request the config-file remotely.
    $file = CONFIG; // the file to which $cache_new gets prepended

    $handle = fopen($file, "r+");
    $len = strlen($cache_new);
    $final_len = filesize($file) + $len;
    $cache_old = fread($handle, $len);
    rewind($handle);
    $i = 1;
    while (ftell($handle) < $final_len) {
        fwrite($handle, $cache_new);
        $cache_new = $cache_old;
        $cache_old = fread($handle, $len);
        fseek($handle, $i * $len);
        $i++;
    }
}

function parse_ini()
{
    $config = new Config_Lite(CONFIG);
    $pageOutput = "<form>";

    $pageOutput .= "<div class='applicationContainer' style='cursor:default;'><h2>General</h2><label for='titleInput'>Title: </label><input id='titleInput' type='text' class='general-value' name='general-title' value='" . $config->get('general', 'title') . "'>";
    $pageOutput .= "<div><label for='dropdownCheckbox'>Enable Dropdown:</label> <input id='dropdownCheckbox' class='general-value' name='general-enabledropdown' type='checkbox' ";
    if ($config->get('general', 'enabledropdown') == true)
        $pageOutput .= "checked></div></div><br><br>";
    else
        $pageOutput .= "></div></div><br>";

    $pageOutput .= "<input type='hidden' class='settings-value' name='settings-enabled' value='true'>" .
        "<input type='hidden' class='settings-value' name='settings-default' value='false'>" .
        "<input type='hidden' class='settings-value' name='settings-name' value='Settings'>" .
        "<input type='hidden' class='settings-value' name='settings-url' value='muximux.php'>" .
        "<input type='hidden' class='settings-value' name='settings-landingpage' value='false'>" .
        "<input type='hidden' class='settings-value' name='settings-icon' value='fa fa-cog'>" .
        "<input type='hidden' class='settings-value' name='settings-dd' value='true'>";

    $pageOutput .= "<div id='sortable'>";
    foreach ($config as $section => $name) {
        if (is_array($name) && $section != "settings" && $section != "general") {
            $pageOutput .= "<div class='applicationContainer' id='" . $section . "'><span class='bars fa fa-bars'></span>";
            foreach ($name as $key => $val) {
                if ($key == "url")
                    $pageOutput .= "<div><label for='" . $section . "-" . $key . "' >URL:</label><input class='" . $section . "-value' name='" . $section . "-" . $key . "' type='text' value='" . $val . "'></div>";
                else if ($key == "name") {
                    $pageOutput .= "<div><label for='" . $section . "-" . $key . "' >Name:</label><input class='appName " . $section . "-value' was='" . $section . "' name='" . $section . "-" . $key . "' type='text' value='" . $val . "'></div>";
                } else if ($key == "icon") {
                    $pageOutput .= "<div><label for='" . $section . "-" . $key . "' >Icon: </label><button class=\"iconpicker btn btn-default\" name='" . $section . "-" . $key . "' data-search=\"true\" data-search-text=\"Search...\"  data-iconset=\"fontawesome\" data-icon=\"" . $val . "\"></button></div>";
                } elseif ($key == "default") {
                    $pageOutput .= "<div><label for='" . $section . "-" . $key . "' >Default:</label><input type='radio' class='radio " . $section . "-value' id='" . $section . "-" . $key . "' name='" . $section . "-" . $key . "'";
                    if ($val == "true")
                        $pageOutput .= " checked></div>";
                    else
                        $pageOutput .= "></div>";
                } else if ($key == "enabled") {
                    $pageOutput .= "<div><label for='" . $section . "-" . $key . "' >Enabled: </label><input class='checkbox " . $section . "-value ' id='" . $section . "-" . $key . "' name='" . $section . "-" . $key . "' type='checkbox' ";
                    if ($val == "true")
                        $pageOutput .= " checked></div>";
                    else
                        $pageOutput .= "></div>";
                } else if ($key == "landingpage") {
                    $pageOutput .= "<div><label for='" . $section . "-" . $key . "' >Enable landing page: </label><input class='checkbox " . $section . "-value' id='" . $section . "-" . $key . "' name='" . $section . "-" . $key . "' type='checkbox' ";
                    if ($val == "true")
                        $pageOutput .= " checked></div>";
                    else
                        $pageOutput .= "></div>";
                } else {
                    $pageOutput .= "<div><label for='" . $section . "-" . $key . "' >Put in dropdown: </label><input class='checkbox " . $section . "-value' id='" . $section . "-" . $key . "' name='" . $section . "-" . $key . "' type='checkbox' ";
                    if ($val == "true")
                        $pageOutput .= " checked></div>";
                    else
                        $pageOutput .= "></div>";
                }
            }

            $pageOutput .= "<button type='button' class='removeButton btn btn-danger btn-xs' value='Remove' id='remove-" . $section . "'>Remove</button></div>"; //Put this back to the left when ajax is ready -- <input type='button' class='saveButton' value='Save' id='save-" . $section . "'>
        }
    }
    $pageOutput .= "</div><div class='center' id='addApplicationButton'>
                    <button type='button' class='btn btn-primary btn-md' id='addApplication'>Add new</button>
                    </form></div>";
    return $pageOutput;
}




function menuItems()
{
    $config = new Config_Lite(CONFIG);
    if (empty($standardmenu)) $standardmenu = '';
    if (empty($dropdownmenu)) $dropdownmenu = '';
    if (empty($enabledropdown)) $enabledropdown = '';
    foreach ($config as $keyname => $section) {
        if (($keyname == "general")) {
            if (isset($section["enabledropdown"]) && ($section["enabledropdown"] == "true")) {
                $enabledropdown = "true";
            } else {
                $enabledropdown = "false";
            }
        }

        if (!empty($section["enabled"]) && !($section["enabled"] == "false") && ($section["enabled"] == "true") && (!isset($section["dd"]) || $section["dd"] == "false")) {
            if (!empty($section["default"]) && !($section["default"] == "false") && ($section["default"] == "true")) {
                $standardmenu .= "<li><a data-content=\"" . $keyname . "\" class=\"selected\"><span class=\"fa " . $section["icon"] . " fa-lg\"></span> " . $section["name"] . "</a></li>\n";
            } else {
                $standardmenu .= "<li><a data-content=\"" . $keyname . "\"><span class=\"fa " . $section["icon"] . " fa-lg\"></span> " . $section["name"] . "</a></li>\n";
            }
        }
        if (isset($section["dd"]) && ($section["dd"] == "true") && !empty($section["enabled"]) && !($section["enabled"] == "false") && ($section["enabled"] == "true") && $section['name'] == "Settings") {
            $dropdownmenu .= "<li><a data-toggle='modal' data-target='#settingsModal'><span class=\"fa " . $section["icon"] . "\"></span> " . $section["name"] . "</a></li>\n";
        } else if (isset($section["dd"]) && ($section["dd"] == "true") && !empty($section["enabled"]) && !($section["enabled"] == "false") && ($section["enabled"] == "true")) {
            $dropdownmenu .= "<li><a data-content=\"" . $keyname . "\" ><span class=\"fa " . $section["icon"] . "\"></span> " . $section["name"] . "</a></li>\n";
        } else {
            $dropdownmenu .= "";
        }
    }

    if ($enabledropdown == "true") {
        $item = "<ul class=\"main-nav\">
        <li class=\"dd\">
        <a><span class=\"fa fa-bars fa-lg\"></span></a>
        <ul class=\"drop-nav\">\n" . $dropdownmenu .
            "</ul></li></ul>\n\n\n<ul class=\"cd-tabs-navigation\"><nav>" .
            $standardmenu .
            "<li><a id=\"reload\" title=\"Double click your app in the menu, or press this button to refresh the current app.\"><span class=\"fa fa-refresh fa-lg\"></span></a></li></ul></nav>";
    } else {
        $item = "<nav><ul class=\"cd-tabs-navigation\">" .
            $standardmenu .
            "<li><a id=\"reload\" title=\"Double click your app in the menu, or press this button to refresh the current app.\"><span class=\"fa fa-refresh fa-lg\"></span></a></li></ul></nav>";
    }
    return $item;
}

function getTitle()
{
    $config = new Config_Lite(CONFIG);
    $item = $config->get('general', 'title');
    if (empty($item)) $item = 'Muximux - Application Management Console';
    return $item;
}


function frameContent()
{
    $config = new Config_Lite(CONFIG);
    if (empty($item)) $item = '';
    foreach ($config as $keyname => $section) {
        if (!empty($section["landingpage"]) && !($section["landingpage"] == "false") && ($section["landingpage"] == "true")) {
            $section["url"] = "?landing=" . $keyname;
        }

        if (!empty($section["enabled"]) && !($section["enabled"] == "false") && ($section["enabled"] == "true")) {
            if (!empty($section["default"]) && !($section["default"] == "false") && ($section["default"] == "true")) {
                $item .= "\n<li data-content=\"" . $keyname . "\" class=\"selected\">\n<iframe sandbox=\"allow-forms allow-same-origin allow-pointer-lock allow-scripts allow-popups allow-modals\" allowfullscreen=\"true\" webkitallowfullscreen=\"true\" mozallowfullscreen=\"true\" scrolling=\"auto\" src=\"" . $section["url"] . "\"></iframe>\n</li>\n";
            } else {
                $item .= "\n<li data-content=\"" . $keyname . "\">\n<iframe sandbox=\"allow-forms allow-same-origin allow-pointer-lock allow-scripts allow-popups allow-modals\" allowfullscreen=\"true\" webkitallowfullscreen=\"true\" mozallowfullscreen=\"true\" scrolling=\"auto\" data-src=\"" . $section["url"] . "\"></iframe>\n</li>\n";
            }

        }
    }
    return $item;
}


function landingPage($keyname)
{
    $config = new Config_Lite(CONFIG);
    $item = "
    <html lang=\"en\">
    <head>
    <title>" . $config->get($keyname, 'name') . "</title>
    <link rel=\"stylesheet\" href=\"css/landing.css\">
    </head>
    <body>
    <div class=\"login\">
        <div class=\"heading\">
            <h2><span class=\"fa " . $config->get($keyname, 'icon') . " fa-3x\"></span></h2>
            <section>
                <a href=\"" . $config->get($keyname, 'url') . "\" target=\"_self\" title=\"Launch " . $config->get($keyname, 'name') . "!\"><button class=\"float\">Launch " . $config->get($keyname, 'name') . "</button></a>
            </section>
        </div>
     </div>
     </body></html>";
    if (empty($item)) $item = '';
    return $item;
}

function command_exist($cmd)
{
    $returnVal = exec("which $cmd");
    return (empty($returnVal) ? false : true);
}

function exec_enabled()
{
    $disabled = explode(', ', ini_get('disable_functions'));
    return !in_array('exec', $disabled);
}


// URL parameters
if (isset($_GET['landing'])) {
    $keyname = $_GET['landing'];
    echo landingPage($keyname);
    die();
}


if (isset($_GET['get']) && $_GET['get'] == 'cwd') {
    echo getcwd();
    die();
}

if (isset($_GET['get']) && $_GET['get'] == 'gitdirectory') {
    $gitdir = getcwd() . "/.git/";
    if (is_readable($gitdir)) {
        echo "readable";
    } else {
        echo "unreadable";
    }
    die();
}

if (isset($_GET['get']) && $_GET['get'] == 'phpini') {
    $inipath = php_ini_loaded_file();

    if ($inipath) {
        echo $inipath;
    } else {
        echo 'php.ini';
    }
    die();
}

if (isset($_GET['get']) && $_GET['get'] == 'hash') {
    if (exec_enabled() == true) {
        if (!command_exist('git')) {
            $hash = 'unknown';
        } else {
            $hash = exec('git log --pretty="%H" -n1 HEAD');
        }
    } else {
        $hash = 'noexec';
    }
    echo $hash;
    die();
}

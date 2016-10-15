<?php
/*
* DO NOT CHANGE THIS FILE!
*/
define('CONFIG', 'settings.ini.php');
define('CONFIGEXAMPLE', 'settings.ini.php-example');
define('SECRET', 'secret.txt');
require dirname(__FILE__) . '/vendor/autoload.php';

// Check if this is an old installation that needs upgrading.
if (file_exists('config.ini.php')) {
    copy('config.ini.php', 'backup.ini.php');
    unlink('config.ini.php');
    $upgrade = true;
} else {
    $upgrade = false;
}

function openFile($file, $mode) {
    if ((file_exists($file) && (!is_writable(dirname($file)) || !is_writable($file))) || !is_writable(dirname($file))) { // If file exists, check both file and directory writeable, else check that the directory is writeable.
        printf('Either the file %s and/or it\'s parent directory is not writable by the PHP process. Check the permissions & ownership and try again.', $file);
        if (PHP_SHLIB_SUFFIX === "so") { //Check for POSIX systems.
            printf("<br>Current permission mode of %s: %d", $file, decoct(fileperms($file) & 0777));
            printf("<br>Current owner of %s: %s", $file, posix_getpwuid(fileowner($filename))['name']);
            printf("<br>Refer to the README on instructions how to change permissions on the aforementioned files.");
        } else if (PHP_SHLIB_SUFFIX === "dll") {
            printf("<br>Detected Windows system, refer to guides on how to set appropriate permissions."); //Can't get fileowner in a trivial manner.
        }

        exit;
    }

    return fopen($file, $mode);
}

function createSecret() {
    $text = uniqid("muximux-", true);
    $file = openFile(SECRET, "w");
    fwrite($file, $text);
    fclose($file);
    return $text;
}

if(!file_exists(CONFIG)){
    copy(CONFIGEXAMPLE, CONFIG);
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
        echo "\n" . 'Exception Message: ' . $e->getMessage();
    }

    $cache_new = "; <?php die(\"Access denied\"); ?>"; // Adds this to the top of the config so that PHP kills the execution if someone tries to request the config-file remotely.
    $file = CONFIG; // the file to which $cache_new gets prepended

    $handle = openFile($file, "r+");
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
    $i=10;
    $scaleRange = "";
    while($i<251) {
        $pr = $i / 100;
        $scaleRange .= "<option value='" . $pr ."'>". $i ."%</option>";
        $i++;
    }
    $config = new Config_Lite(CONFIG);

    if ($config->get('general', 'branch', 'master') == "master") {
        $master = "<option value=\"master\" selected>master</option>";
    } else { $master = "<option value=\"master\">master</option>"; }

    if ($config->get('general', 'branch', 'master') == "develop") {
        $develop = "<option value=\"develop\" selected>develop</option>";
    } else { $develop = "<option value=\"develop\">develop</option>"; }

    if ($config->get('general', 'updatepopup', 'false') == "true") {
        $showUpdates = "<div><label for='updatepopupCheckbox'>Enable update poups:</label> <input id='updatepopupCheckbox' class='general-value' name='general-updatepopup' type='checkbox' checked></div>";
    } else { $showUpdates = "<div><label for='updatepopupCheckbox'>Enable update poups:</label> <input id='updatepopupCheckbox' class='general-value' name='general-updatepopup' type='checkbox'></div>"; }

    $pageOutput = "<form>";

    $pageOutput .= "<div class='applicationContainer' style='cursor:default;'><h2>General</h2><label for='titleInput'>Title: </label><input id='titleInput' type='text' class='general-value' name='general-title' value='" . $config->get('general', 'title', 'Muximux - Application Management Console') . "'>";
    $pageOutput .= "<label for=\"branch\">Branch tracking:</label><select id=\"branch\" name='general-branch'>$master $develop</select>";
    $pageOutput .= "<div><label for='dropdownCheckbox'>Enable Dropdown:</label> <input id='dropdownCheckbox' class='checkbox general-value' name='general-enabledropdown' type='checkbox' ";
    if ($config->get('general', 'enabledropdown') == true)
        $pageOutput .= "checked></div>$showUpdates</div><br><br>";
    else
        $pageOutput .= "></div>$showUpdates</div><br>";

    $pageOutput .= "<input type='hidden' class='settings-value' name='settings-enabled' value='true'>" .
        "<input type='hidden' class='settings-value' name='settings-default' value='false'>" .
        "<input type='hidden' class='settings-value' name='settings-name' value='Settings'>" .
        "<input type='hidden' class='settings-value' name='settings-url' value='muximux.php'>" .
        "<input type='hidden' class='settings-value' name='settings-landingpage' value='false'>" .
        "<input type='hidden' class='settings-value' name='settings-icon' value='fa-cog'>" .
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
                    $pageOutput .= "<div><label for='" . $section . "-" . $key . "' >Icon: </label><button role=\"iconpicker\" class=\"iconpicker btn btn-default\" name='" . $section . "-" . $key . "' data-rows=\"4\" data-cols=\"6\" data-search=\"true\" data-search-text=\"Search...\" data-iconset=\"fontawesome\" data-placement=\"left\" data-icon=\"" . $val . "\"></button></div>";
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
                } else if ($key == "dd") {
                    $pageOutput .= "<div><label for='" . $section . "-dd'>Put in dropdown: </label><input class='checkbox " . $section . "-value' id='" . $section . "-dd' name='" . $section . "-dd' type='checkbox' ";
                    if ($val == "true")
                        $pageOutput .= " checked></div>";
                    else
                        $pageOutput .= "></div>";
                }
            }
            $pageOutput .= "
            <div style=\"margin-left:5px;\"><label for='" . $section . "-scale'>Zoom: </label>
            <select id='" . $section . "-scale' name='" . $section . "-scale'>";

            $pageOutput .= $scaleRange ."</select></div>\n<button type='button' class='removeButton btn btn-danger btn-xs' value='Remove' id='remove-" . $section . "'>Remove</button></div>";
        }
    }
    $pageOutput .= "</div>
    <div class='text-center' style='margin-top: 15px;'>
    <div class='btn-group' role='group' aria-label='Buttons'>
                    <a class='btn btn-primary btn-md' id='addApplication'><span class='fa fa-plus'></span> Add new</a>
                    <a class='btn btn-danger btn-md' id='removeAll'><span class='fa fa-trash'></span> Remove all</a>
                    </div></div></form>";
    return $pageOutput;
}


function menuItems()
{
    $config = new Config_Lite(CONFIG);
    $standardmenu = "";
    $dropdownmenu = "";

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
                $standardmenu .= "<li class='cd-tab'><a data-content=\"" . $keyname . "\" data-title=\"" . $section["name"] . "\" class=\"selected\"><span class=\"fa " . $section["icon"] . " fa-lg\"></span> " . $section["name"] . "</a></li>\n";
            } else {
                $standardmenu .= "<li class='cd-tab'><a data-content=\"" . $keyname . "\" data-title=\"" . $section["name"] . "\"><span class=\"fa " . $section["icon"] . " fa-lg\"></span> " . $section["name"] . "</a></li>\n";
            }
        }
        if (isset($section["dd"]) && ($section["dd"] == "true") && !empty($section["enabled"]) && !($section["enabled"] == "false") && ($section["enabled"] == "true") && $section['name'] == "Settings") {
            $dropdownmenu .= "<li><a data-toggle=\"modal\" data-target=\"#settingsModal\" data-title=\"Settings\"><span class=\"fa " . $section["icon"] . "\"></span> " . $section["name"] . "</a></li>\n";
        } else if (isset($section["dd"]) && ($section["dd"] == "true") && !empty($section["enabled"]) && !($section["enabled"] == "false") && ($section["enabled"] == "true")) {
            $dropdownmenu .= "<li><a data-content=\"" . $keyname . "\" data-title=\"" . $section["name"] . "\"><span class=\"fa " . $section["icon"] . "\"></span> " . $section["name"] . "</a></li>\n";
        } else {
            $dropdownmenu .= "";
        }
    }

    if ($enabledropdown == "true") {
        $item = "<ul class=\"main-nav\">
        <li class=\"dd\">
        <a id=\"hamburger\"><span class=\"fa fa-bars fa-lg\"></span></a>
        <ul class=\"drop-nav\">\n" . $dropdownmenu .
            "</ul></li></ul>\n\n\n<ul class=\"cd-tabs-navigation\"><nav>" .
            $standardmenu .
            "<li class='cd-tab'><a id=\"reload\" title=\"Double click your app in the menu, or press this button to refresh the current app.\"><span class=\"fa fa-refresh fa-lg\"></span></a></li></ul></nav>";
    } else {
        $item = "<nav><ul class=\"cd-tabs-navigation\">" .
            $standardmenu .
            "<li class='cd-tab'><a id=\"reload\" title=\"Double click your app in the menu, or press this button to refresh the current app.\"><span class=\"fa fa-refresh fa-lg\"></span></a></li></ul></nav>";
    }
    return $item;
}

function getTitle()
{
    $config = new Config_Lite(CONFIG);
    $item = $config->get('general', 'title', 'Muximux - Application Management Console');
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

        if (empty($section["scale"]) or ($section["scale"] == "false")) {
            $section["scale"] = 1;
        }

        if (!empty($section["enabled"]) && !($section["enabled"] == "false") && ($section["enabled"] == "true")) {
            if (!empty($section["default"]) && !($section["default"] == "false") && ($section["default"] == "true")) {
                $item .= "\n<li data-content=\"" . $keyname . "\" data-scale=\"" . $section["scale"] ."\" class=\"selected\">\n<iframe sandbox=\"allow-forms allow-same-origin allow-pointer-lock allow-scripts allow-popups allow-modals allow-top-navigation\" allowfullscreen=\"true\" webkitallowfullscreen=\"true\" mozallowfullscreen=\"true\" scrolling=\"auto\" data-title=\"" . $section["name"] . "\" src=\"" . $section["url"] . "\"></iframe>\n</li>\n";
            } else {
                $item .= "\n<li data-content=\"" . $keyname . "\" data-scale=\"" . $section["scale"] ."\">\n<iframe sandbox=\"allow-forms allow-same-origin allow-pointer-lock allow-scripts allow-popups allow-modals allow-top-navigation\" allowfullscreen=\"true\" webkitallowfullscreen=\"true\" mozallowfullscreen=\"true\" scrolling=\"auto\" data-title=\"" . $section["name"] . "\" data-src=\"" . $section["url"] . "\"></iframe>\n</li>\n";
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


// This is where the JavaScript reads the contents of the secret file. This gets re-generated on each page load.
if (isset($_GET['get']) && $_GET['get'] == 'secret') {
        $secret = file_get_contents(SECRET) or die("Unable to open " . SECRET);
        echo $secret;
        die();
}

    // What branch does the user want to track?
if (isset($_GET['get']) && $_GET['get'] == 'branch') {
    $config = new Config_Lite(CONFIG);
    echo $config->get('general', 'branch', 'master');
    die();
}




// Things wrapped inside this are protected by a secret hash.
if(isset($_GET['secret']) && $_GET['secret'] == file_get_contents(SECRET)) {


    // This lets us create a new secret when we leave the page.
    if (isset($_GET['set']) && $_GET['set'] == 'secret') {
            createSecret();
            die();
    }

    // Get the local path where this script is running from.
    if (isset($_GET['get']) && $_GET['get'] == 'cwd') {
        echo getcwd();
        die();
    }

    // Get the title configured in settings.php.ini
    if (isset($_GET['get']) && $_GET['get'] == 'title') {
        $config = new Config_Lite(CONFIG);
        echo $config->get('general', 'title', 'Muximux');
        die();
    }



    // Determine if there is a .git directory and if it's readable.
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

    if (isset($_GET['get']) && $_GET['get'] == 'greeting') {
        $config = new Config_Lite(CONFIG);
        echo $config->get('general', 'greeting', 'false');
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

    if(isset($_GET['remove']) && $_GET['remove'] == "backup") {
        unlink('backup.ini.php');
        echo "deleted";
        die();
    }
}
// End protected get-calls


if(empty($_GET)) {
    createSecret();
}
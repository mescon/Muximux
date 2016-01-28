<?php
/**
 * Created by PhpStorm.
 * User: synik
 * Date: 1/18/2016
 * Time: 9:19 PM
 * Hello CauseFX - don't touch please!
 */
require __DIR__ . '/vendor/autoload.php';

if (sizeof($_POST) > 0){
    write_ini();
    $cache_new = "; <?php die(\"Access denied\"); ?>"; // Adds this to the top of the config so that PHP kills the execution if someone tries to request the config-file remotely.
    $file = "settings.ini.php"; // the file to which $cache_new gets prepended

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

else
    parse_ini();




function write_ini()
{
    unlink('settings.ini.php');

    $config = new Config_Lite('settings.ini.php');
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
}


function parse_ini()
{
    $config = new Config_Lite('settings.ini.php');
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
        "<input type='hidden' class='settings-value' name='settings-url' value='settings.php'>" .
        "<input type='hidden' class='settings-value' name='settings-landingpage' value='false'>" .
        "<input type='hidden' class='settings-value' name='settings-icon' value='fa fa-server'>" .
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
                    $pageOutput .= "<div><label for='" . $section . "-" . $key . "' >Icon: </label><button class=\"iconpicker btn btn-default\" name='" . $section . "-" . $key . "' data-search=\"true\" data-search-text=\"Search...\"  data-iconset=\"fontawesome\" data-icon=\"".$val."\"></button></div>";
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
                    $pageOutput .= "<div><label for='". $section . "-" . $key."' >Put in dropdown: </label><input class='checkbox " . $section . "-value' id='" . $section . "-" . $key . "' name='" . $section . "-" . $key . "' type='checkbox' ";
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
                    <div id='saved'>Saved!</div><div id='removed' class='hidden'></div></form></div>
                    <meta id='gitData'><meta id='versionText'>";
    return $pageOutput;
}
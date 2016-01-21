<?php
/**
 * Created by PhpStorm.
 * User: synik
 * Date: 1/18/2016
 * Time: 9:19 PM
 */
require __DIR__ . '/vendor/autoload.php';

if (sizeof($_POST) == 0) {
    parse_ini();
} else {
    save_ini();
}

function save_ini()
{
    unlink('config.ini.php');
    $config = new Config_Lite('config.ini.php');
    foreach ($_POST as $parameter => $value) {
        if ($parameter != "ignore") {
            $splitParameter = explode('-', $parameter);
            if ($value == "on")
                $value = "true";
            $config->set($splitParameter[0], $splitParameter[1], $value);
        }
    }
    // save object to file
    try {
        $config->save();
    } catch (Config_Lite_Exception $e) {
        echo "\n", 'Exception Message: ', $e->getMessage();
    }
    parse_ini();
}

function parse_ini()
{
    $config = new Config_Lite('config.ini.php');
    $pageOutput = "<div id='header'>Settings</div>";

    $pageOutput .= "<form method='post' action='settings.php'>";

    $pageOutput .= "<div class='applicationContainer'>General:<br>Title: <input type='text' class='general-value' name='general-title' value='" . $config->get('general', 'title') . "'>";
    $pageOutput .= "Enable Dropdown: <input class='general-value' name='general-enabledropdown' type='checkbox' ";
    if ($config->get('general', 'enabledropdown') == true)
        $pageOutput .= "checked></div><br><br>";
    else
        $pageOutput .= "></div><br>";

    $pageOutput .= "<input type='hidden' class='settings-value' name='settings-enabled' value='true'>".
                   "<input type='hidden' class='settings-value' name='settings-default' value='false'>".
                   "<input type='hidden' class='settings-value' name='settings-name' value='Settings'>".
                   "<input type='hidden' class='settings-value' name='settings-url' value='settings.php'>".
                   "<input type='hidden' class='settings-value' name='settings-landingpage' value='false'>".
                   "<input type='hidden' class='settings-value' name='settings-icon' value='fa fa-server'>".
                   "<input type='hidden' class='settings-value' name='settings-dd' value='true'>";

    $pageOutput .= "<div class='center' id='addApplicationButton'><input type='button' id='addApplication' value='Add New Application'></div><ul id='sortable'>";
    foreach ($config as $section => $name) {
        if (is_array($name) && $section != "settings" && $section != "general") {
            $pageOutput .= "<li class='applicationContainer' id='" . $section . "'><div>Application: <input class='applicationName' was='" . $section . "' type='text' value='" . $section . "'><input type='button' class='saveApp' value='Update Application Name'></div>";
            foreach ($name as $key => $val) {
                if ($key == "name" || $key == "url" || $key == "icon")
                    $pageOutput .= "<div>$key:<input class='" . $section . "-value' name='" . $section . "-" . $key . "' type='text' value='" . $val . "'></div></div>";
                else {
                    $pageOutput .= "<div>$key:<input class='checkbox " . $section . "-value' name='" . $section . "-" . $key . "' type='checkbox' ";
                    if ($val == "true")
                        $pageOutput .= " checked></div></div>";
                    else
                        $pageOutput .= "></div></div>";
                }
            }

            $pageOutput .= "<input type='button' class='removeButton' value='Remove' id='remove-" . $section . "'></li>"; //Put this back to the left when ajax is ready -- <input type='button' class='saveButton' value='Save' id='save-" . $section . "'>
        }
    }
    $pageOutput .= "</ul><input type='submit' id='settingsSubmit' value='Submit Changes'></form>";
    echo $pageOutput;
}

?>
<html>
<head>
    <script src="js/jquery-2.2.0.min.js"></script>
    <script src="js/jquery-ui.min.js"></script>
    <script src="js/main.js"></script>
    <!-- Resource jQuery -->
    <link rel="stylesheet" href="css/jquery-ui.min.css">
    <link rel="stylesheet" href="css/settingsStyle.css">
    <link rel="stylesheet" href="//fonts.googleapis.com/css?family=PT+Sans:400" type="text/css">
    <!-- Font -->
</head>
</html>

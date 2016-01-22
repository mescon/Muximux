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
    write_ini();
}

function write_ini()
{
    $config = new Config_Lite('config.ini.php');
    foreach ($_POST as $parameter => $value) {
        $splitParameter = explode('-', $parameter);
        if ($splitParameter[0] != "removed") {
            if ($value == "on")
                $value = "true";
            $config->set($splitParameter[0], $splitParameter[1], $value);
        } else
            $config->removeSection($splitParameter[1]);
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
    $iconVal = array('fa fa-television', 'fa fa-download', 'glyphicon glyphicon-calendar', 'glyphicon glyphicon-dashboard', 'glyphicon glyphicon-bullhorn', 'fa fa-server', 'fa fa-play-circle');

    $pageOutput = "<div id='header'>Settings</div><form method='post' action='settings.php'>";

    $pageOutput .= "<div class='applicationContainer'>General:<br>Title: <input type='text' class='general-value' name='general-title' value='" . $config->get('general', 'title') . "'>";
    $pageOutput .= "<div>Enable Dropdown: <input class='general-value' name='general-enabledropdown' type='checkbox' ";
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
            $pageOutput .= "<div class='applicationContainer' id='" . $section . "'>
            <input type='button' class='saveApp' value='Update Application Name'>
            <span class='example_icon'></span>";
            foreach ($name as $key => $val) {
                if ($key == "url")
                    $pageOutput .= "<div>$key:<input class='" . $section . "-value' name='" . $section . "-" . $key . "' type='text' value='" . $val . "'></div>";
                else if ($key == "name") {
                    $pageOutput .= "<div>$key:<input class='appName " . $section . "-value' was='" . $section . "' name='" . $section . "-" . $key . "' type='text' value='" . $val . "'></div>";
                } else if ($key == "icon") {
                    $pageOutput .= "<div>$key:<select class='iconDD " . $section . "-value' name='" . $section . "-" . $key . "'>";
                    foreach ($iconVal as $icon) {
                        $pageOutput .= "<option value='" . $icon . "'" . ($val == $icon ? " selected>" : ">") . $icon . "</option>";
                    }
                    $pageOutput .= "</select></div>";
                } elseif ($key == "default") {
                        $pageOutput .= "<div>$key:<input type='radio' class='radio ".$section."-value' name='" . $section . "-" . $key . "'";
                    if($val == "true")
                        $pageOutput.= " checked></div>";
                    else
                        $pageOutput.= "></div>";
                } else {
                    $pageOutput .= "<div>$key:<input class='checkbox " . $section . "-value' name='" . $section . "-" . $key . "' type='checkbox' ";
                    if ($val == "true")
                        $pageOutput .= " checked></div>";
                    else
                        $pageOutput .= "></div>";
                }
            }

            $pageOutput .= "<input type='button' class='removeButton' value='Remove' id='remove-" . $section . "'></div>"; //Put this back to the left when ajax is ready -- <input type='button' class='saveButton' value='Save' id='save-" . $section . "'>
        }
    }
    $pageOutput .= "</div><input type='submit' id='settingsSubmit' value='Submit Changes'><div id='removed' class='hidden'></div></form>";
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
    <link rel="stylesheet" href="//fonts.googleapis.com/css?family=PT+Sans:400" type="text/css">
    <link rel="stylesheet" href="https://maxcdn.bootstrapcdn.com/bootstrap/3.3.6/css/bootstrap.min.css"
          integrity="sha384-1q8mTJOASx8j1Au+a5WDVnPi2lkFfwwEAa8hDDdjZlpLegxhjVME1fgjWPGmkzs7" crossorigin="anonymous">
    <!-- Bootstrap (includes Glyphicons) -->
    <link rel="stylesheet" href="css/settingsStyle.css">
    <!-- Optional theme -->
    <link rel="stylesheet" href="https://maxcdn.bootstrapcdn.com/bootstrap/3.3.6/css/bootstrap-theme.min.css"
          integrity="sha384-fLW2N01lMqjakBkx3l/M9EahuwpSfeNvV63J5ezn3uZzapT0u7EYsXMjQV+0En5r" crossorigin="anonymous">
    <link rel="stylesheet" href="https://maxcdn.bootstrapcdn.com/font-awesome/4.5.0/css/font-awesome.min.css"/>
    <!--FontAwesome-->

    <!-- Font -->
</head>
</html>

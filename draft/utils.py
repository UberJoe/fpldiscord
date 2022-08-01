import getpass
import requests
import json
import pandas as pd
import config as cnf
from boto.s3.connection import S3Connection

def get_json():
    """
    Pulls fpl draft league data using an api call, and stores the output
    in a json file at the specified location.
    
    To get the FPL Draft api call, I followed advice on [Reddit here](https://www.reddit.com/r/FantasyPL/comments/9rclpj/python_for_fantasy_football_using_the_fpl_api/e8g6ur4?utm_source=share&utm_medium=web2x) which basically said you can derive the API calls by using Chrome's developer window under network you can see the "fetches" that are made, and the example response data. Very cool!
    
    :param file_path: The file path and name of the json file you wish to create
    :param api: The api call for your fpl draft league
    :param email_address: Your email address to authenticate with premierleague.com
    :returns: 
    """
    json_files = [
        '../data/transactions.json',
        '../data/elements.json',
        '../data/details.json',
        '../data/element_status.json'
    ]
    
    apis = [
        'https://draft.premierleague.com/api/draft/league/36298/transactions',
        'https://draft.premierleague.com/api/bootstrap-static',
        'https://draft.premierleague.com/api/league/36298/details',
        'https://draft.premierleague.com/api/league/36298/element-status'
    ]
    
    # Post credentials for authentication
    # pwd = getpass.getpass('Enter Password: ')
    session = requests.session()
    url = 'https://users.premierleague.com/accounts/login/'
    payload = {
     'password': S3Connection(os.environ('pwd')),
     'login': S3Connection(os.environ('email')),
     'redirect_uri': 'https://fantasy.premierleague.com/a/login',
     'app': 'plfpl-web'
    }
    session.post(url, data=payload)
    
    # Loop over the api(s), call them and capture the response(s)
    for file, i in zip(json_files, apis):
        r = session.get(i)
        jsonResponse = r.json()
        with open(file, 'w') as outfile:
            json.dump(jsonResponse, outfile)

def get_data(df_name):
    # Dataframes from the details.json
    if df_name == 'league_entries':
        with open('../data/details.json') as json_data:
            d = json.load(json_data)
            league_entry_df = pd.json_normalize(d['league_entries'])
            
        return league_entry_df
    
    elif df_name == 'matches':
        with open('../data/details.json') as json_data:
            d = json.load(json_data)
            matches_df = pd.json_normalize(d['matches'])
            
        return matches_df
    
    elif df_name == 'standings':
        with open('../data/details.json') as json_data:
            d = json.load(json_data)
            standings_df = pd.json_normalize(d['standings'])
            
        return standings_df
    
    # Dataframes from the elements.json
    elif df_name == 'elements':
        with open('../data/elements.json') as json_data:
            d = json.load(json_data)
            elements_df = pd.json_normalize(d['elements'])
            
        return elements_df
    
    elif df_name == 'element_types':
        with open('../data/elements.json') as json_data:
            d = json.load(json_data)
            element_types_df = pd.json_normalize(d['element_types'])
            
        return element_types_df
    
    # Dataframes from the transactions.json
    elif df_name == 'transactions':
        with open('../data/transactions.json') as json_data:
            d = json.load(json_data)
            transactions_df = pd.json_normalize(d['transactions'])
            
        return transactions_df
    
    # Dataframes from the element_status.json
    elif df_name == 'element_status':
        with open('../data/element_status.json') as json_data:
            d = json.load(json_data)
            element_status_df = pd.json_normalize(d['element_status'])
            
        return element_status_df
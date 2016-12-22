service KeyValueService {
    void Set(1: string key, 2: string value)
    string Get(1: string key)
    list<string> GetAll (1: list<string> keys)
}
